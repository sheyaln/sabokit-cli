package cli

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/sheyaln/sabokit-cli/internal/docker"
	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var apps, servers []string
	var dryRun, skipRefresh bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "enabled apps + container state per host",
		Long: `prints two sections:

  1. enabled apps — reads <env>/.enabled_apps.json (written by the layer
     scripts / 'sabokit refresh') and lists each app with its URL.

  2. container state — runs 'ansible all -m shell -a "docker ps ..."' over
     ssh against the env's inventory. --servers passes through to ansible
     --limit. --apps post-filters the container listing.

both sections are refreshed from terraform state first unless
--skip-refresh is passed.`,
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
	cmd.Flags().BoolVar(&skipRefresh, "skip-refresh", false, "skip regenerating inventory.ini + .enabled_apps.json first")
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
		if envName := p.EnvName(globals.Env); envName != "" {
			if err := runScript(p, "refresh.sh", envName); err != nil {
				return err
			}
		}
	}

	fmt.Println("== enabled apps ==")
	printEnabledAppsSection(p)
	fmt.Println()

	fmt.Println("== container state ==")
	return printContainerState(p, apps, servers)
}

func printEnabledAppsSection(p *project.Project) {
	apps, err := p.EnvApps(globals.Env)
	if err != nil {
		fmt.Printf("(%v)\n", err)
		return
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].ID < apps[j].ID })
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "APP\tURL")
	for _, a := range apps {
		if !a.Enabled {
			continue
		}
		fmt.Fprintf(tw, "%s\t%s\n", a.ID, a.URL)
	}
	tw.Flush()
}

func containerStateInvocation(p *project.Project, servers []string) (docker.Invocation, error) {
	envName := p.EnvName(globals.Env)
	if envName == "" {
		return docker.Invocation{}, fmt.Errorf("status requires an env (pass --env or set default_env in .sabokit/config.yml)")
	}
	cmd := []string{
		"all",
		"-i", "environments/" + envName + "/" + p.Config.Inventory,
		"-m", "shell",
		"-a", "docker ps --format '{{.Names}}\t{{.Status}}'",
		"-o",
	}
	if len(servers) > 0 {
		cmd = append(cmd, "--limit", strings.Join(servers, ","))
	}
	inv, err := repoInvocation(p)
	if err != nil {
		return docker.Invocation{}, err
	}
	inv.TTY = false
	inv.Entrypoint = "ansible"
	inv.Cmd = cmd
	inv.Env["ANSIBLE_HOST_KEY_CHECKING"] = "False"
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
