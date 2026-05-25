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
