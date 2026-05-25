package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sheyaln/sabokit-cli/internal/docker"
	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/spf13/cobra"
)

func newDownCmd() *cobra.Command {
	var apps, servers []string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "down",
		Short: "stop apps (docker compose down) without destroying cloud resources",
		Long: `runs ansible-playbook down.yml --tags <apps> inside the runner image. each
named app's containers are stopped via docker compose down on the host
running it. cloud resources (instances, dns, secrets, TF state) are not
touched. reversible via 'sabokit deploy --apps <same>'.

--apps is required; pass one or more app names. --servers narrows ansible
--limit to specific hosts (useful when an app is split across hosts).

requires the sabokit-runner v3.0.0+ image (down.yml does not exist in older
runner builds).`,
		Example: `  # stop one app on all hosts that run it
  sabokit down --apps espocrm

  # stop multiple apps, restricted to one host
  sabokit down --apps espocrm,n8n --servers app01

  # preview the docker invocation
  sabokit down --apps espocrm --print`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(apps) == 0 {
				return fmt.Errorf("--apps is required")
			}
			return runDown(apps, servers, dryRun)
		},
	}
	cmd.Flags().StringSliceVar(&apps, "apps", nil, "apps to stop (required)")
	cmd.Flags().StringSliceVar(&servers, "servers", nil, "restrict to specific hosts")
	cmd.Flags().BoolVar(&dryRun, "print", false, "print the docker invocation without running it")
	return cmd
}

func runDown(apps, servers []string, dryRun bool) error {
	p, err := project.Load()
	if err != nil {
		return err
	}

	cmd := []string{
		"down.yml",
		"-i", filepath.Join(containerWorkspace, p.Config.Inventory),
		"--tags", strings.Join(apps, ","),
	}
	if len(servers) > 0 {
		cmd = append(cmd, "--limit", strings.Join(servers, ","))
	}
	if globals.Verbose {
		cmd = append(cmd, "-v")
	}

	inv := baseInvocation(p)
	inv.Workdir = playbookDir
	inv.Entrypoint = "ansible-playbook"
	inv.Cmd = cmd

	if dryRun {
		fmt.Println("docker", strings.Join(inv.Args(), " "))
		return nil
	}
	if err := docker.Preflight(); err != nil {
		return err
	}
	return inv.Command().Run()
}
