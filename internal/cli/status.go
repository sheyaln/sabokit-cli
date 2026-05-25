package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/sheyaln/sabokit-cli/internal/docker"
	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var apps, servers []string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "terraform output + container state per host",
		Long: `prints two sections:

  1. terraform outputs — runs 'terraform output -json' inside the runner
     image for each layer in order: base, identity, apps. uninitialized
     layers are shown as <not initialized>. sensitive outputs are masked.

  2. container state — runs 'ansible all -m shell -a "docker ps ..."' over
     ssh against the project inventory. --servers passes through to
     ansible --limit. --apps post-filters the container listing by name
     (does NOT filter the terraform section).

requires docker live. ssh transport runs inside the runner image; the
project's ssh key (from .sabokit/config.yml) is mounted read-only.`,
		Example: `  # full status across all layers and hosts
  sabokit status

  # narrow to one host
  sabokit status --servers app01

  # show only containers whose name contains 'espocrm' or 'authentik'
  sabokit status --apps espocrm,authentik`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(apps, servers)
		},
	}
	cmd.Flags().StringSliceVar(&apps, "apps", nil, "filter container list by app name")
	cmd.Flags().StringSliceVar(&servers, "servers", nil, "restrict to specific hosts")
	return cmd
}

type tfOutput struct {
	Value     any  `json:"value"`
	Sensitive bool `json:"sensitive"`
}

func runStatus(apps, servers []string) error {
	p, err := project.Load()
	if err != nil {
		return err
	}
	if err := docker.Preflight(); err != nil {
		return err
	}

	fmt.Println("== terraform outputs ==")
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "LAYER\tKEY\tVALUE")
	for _, layer := range []string{"base", "identity", "apps"} {
		printLayerOutputs(tw, p, layer)
	}
	tw.Flush()
	fmt.Println()

	fmt.Println("== container state ==")
	return printContainerState(p, apps, servers)
}

func printLayerOutputs(tw *tabwriter.Writer, p *project.Project, layer string) {
	inv := baseInvocation(p)
	inv.TTY = false
	inv.Workdir = filepath.Join(terraformDir, layer)
	inv.Entrypoint = "terraform"
	inv.Cmd = []string{"output", "-json"}

	var stdout bytes.Buffer
	c := inv.Command()
	c.Stdout = &stdout
	c.Stderr = nil
	if err := c.Run(); err != nil {
		fmt.Fprintf(tw, "%s\t-\t<not initialized>\n", layer)
		return
	}
	var outputs map[string]tfOutput
	if err := json.Unmarshal(stdout.Bytes(), &outputs); err != nil {
		fmt.Fprintf(tw, "%s\t-\t<parse error>\n", layer)
		return
	}
	if len(outputs) == 0 {
		fmt.Fprintf(tw, "%s\t-\t<no outputs>\n", layer)
		return
	}
	for k, o := range outputs {
		val := formatTfValue(o)
		fmt.Fprintf(tw, "%s\t%s\t%s\n", layer, k, val)
	}
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
		return string(b)
	}
}

func printContainerState(p *project.Project, apps, servers []string) error {
	cmd := []string{
		"all",
		"-i", filepath.Join(containerWorkspace, p.Config.Inventory),
		"-m", "shell",
		"-a", "docker ps --format '{{.Names}}\t{{.Status}}'",
		"-o",
	}
	if len(servers) > 0 {
		cmd = append(cmd, "--limit", strings.Join(servers, ","))
	}

	inv := baseInvocation(p)
	inv.TTY = false
	inv.Workdir = playbookDir
	inv.Entrypoint = "ansible"
	inv.Cmd = cmd

	var stdout bytes.Buffer
	c := inv.Command()
	c.Stdout = &stdout
	c.Stderr = os.Stderr
	err := c.Run()
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
