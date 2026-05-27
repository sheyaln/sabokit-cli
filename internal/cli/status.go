package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/sheyaln/sabokit-cli/internal/docker"
	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/sheyaln/sabokit-cli/internal/tf"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var apps, servers []string
	var dryRun, skipRefresh bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "terraform output + container state per host",
		Long: `prints two sections:

  1. terraform outputs — reads <env>/.tf-output.json (written by
     consumer-template's up.sh) and prints the top-level keys as a table.
     this does NOT shell out to terraform; sabokit relies on the env's
     existing snapshot. if .tf-output.json is missing, the section is
     skipped with a hint to run up.sh.

  2. container state — runs 'ansible all -m shell -a "docker ps ..."' over
     ssh against the env's inventory. --servers passes through to ansible
     --limit. --apps post-filters the container listing (does NOT filter
     the terraform section).

requires docker live for the container-state section only. tf section is
host-side json parsing.`,
		Example: `  sabokit status
  sabokit status --servers app01
  sabokit status --apps espocrm,authentik
  sabokit status --print     # docker invocation only`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(apps, servers, dryRun, skipRefresh)
		},
	}
	cmd.Flags().StringSliceVar(&apps, "apps", nil, "filter container list by app name")
	cmd.Flags().StringSliceVar(&servers, "servers", nil, "restrict to specific hosts")
	cmd.Flags().BoolVar(&dryRun, "print", false, "print the docker invocation without running it")
	cmd.Flags().BoolVar(&skipRefresh, "skip-refresh", false, "skip re-running terraform output to refresh .tf-output.json + inventory.ini")
	return cmd
}

func runStatus(apps, servers []string, dryRun, skipRefresh bool) error {
	p, err := project.Load()
	if err != nil {
		return err
	}

	if dryRun {
		inv, err := containerStateInvocation(p, servers)
		if err != nil {
			return err
		}
		fmt.Println("docker", strings.Join(inv.Args(), " "))
		return nil
	}

	if err := docker.Preflight(); err != nil {
		return err
	}

	if !skipRefresh {
		if err := refreshIfEnv(p, tf.New(globals.TFImage, globals.Platform)); err != nil {
			return err
		}
	}

	fmt.Println("== terraform outputs ==")
	printTFOutputs(p, globals.Env)
	fmt.Println()

	fmt.Println("== container state ==")
	return printContainerState(p, apps, servers)
}

type tfOutput struct {
	Value     any  `json:"value"`
	Sensitive bool `json:"sensitive"`
}

func printTFOutputs(p *project.Project, envOverride string) {
	path := p.TFOutputPath(envOverride)
	if path == "" {
		fmt.Println("(no env set — skipping; pass --env or set default_env in .sabokit/config.yml)")
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("(no .tf-output.json at %s — run up.sh in the env dir to generate)\n", path)
		return
	}
	var outputs map[string]tfOutput
	if err := json.Unmarshal(raw, &outputs); err != nil {
		fmt.Printf("(parse error in %s: %v)\n", path, err)
		return
	}
	if len(outputs) == 0 {
		fmt.Println("(empty tf outputs)")
		return
	}

	keys := make([]string, 0, len(outputs))
	for k := range outputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KEY\tVALUE")
	for _, k := range keys {
		fmt.Fprintf(tw, "%s\t%s\n", k, formatTfValue(outputs[k]))
	}
	tw.Flush()
}

func formatTfValue(o tfOutput) string {
	if o.Sensitive {
		return "<sensitive>"
	}
	switch v := o.Value.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		b, _ := json.Marshal(v)
		s := string(b)
		if len(s) > 120 {
			s = s[:117] + "..."
		}
		return s
	}
}

func containerStateInvocation(p *project.Project, servers []string) (docker.Invocation, error) {
	cmd := []string{
		"all",
		"-i", containerWorkspace + "/" + p.Config.Inventory,
		"-m", "shell",
		"-a", "docker ps --format '{{.Names}}\t{{.Status}}'",
		"-o",
	}
	if len(servers) > 0 {
		cmd = append(cmd, "--limit", strings.Join(servers, ","))
	}
	inv, err := baseInvocation(p)
	if err != nil {
		return docker.Invocation{}, err
	}
	inv.TTY = false
	inv.Workdir = playbookDir
	inv.Entrypoint = "ansible"
	inv.Cmd = cmd
	return inv, nil
}

func printContainerState(p *project.Project, apps, servers []string) error {
	inv, err := containerStateInvocation(p, servers)
	if err != nil {
		return err
	}
	var stdout bytes.Buffer
	c := inv.Command()
	c.Stdout = &stdout
	c.Stderr = os.Stderr
	err = c.Run()
	out := stdout.String()
	if len(apps) > 0 {
		out = filterByApps(out, apps)
	}
	fmt.Print(out)
	return err
}

func filterByApps(out string, apps []string) string {
	var b strings.Builder
	for _, line := range strings.Split(out, "\n") {
		for _, a := range apps {
			if strings.Contains(line, a) {
				b.WriteString(line)
				b.WriteString("\n")
				break
			}
		}
	}
	return b.String()
}
