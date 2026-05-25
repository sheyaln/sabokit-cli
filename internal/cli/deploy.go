package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sheyaln/sabokit-cli/internal/docker"
	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/spf13/cobra"
)

type deployFlags struct {
	apps          []string
	servers       []string
	base          bool
	noBase        bool
	rotateSecrets bool
	check         bool
	overlay       string
	dryRun        bool
}

func newDeployCmd() *cobra.Command {
	f := &deployFlags{}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "run ansible against the project (ports fc-runner.sh flag shape)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if f.base && f.noBase {
				return fmt.Errorf("--base and --no-base are mutually exclusive")
			}
			return runDeploy(f)
		},
	}
	cmd.Flags().StringSliceVar(&f.apps, "apps", nil, "restrict to specific apps (ansible --tags)")
	cmd.Flags().StringSliceVar(&f.servers, "servers", nil, "restrict to specific hosts (ansible --limit)")
	cmd.Flags().BoolVar(&f.base, "base", false, "include base host roles")
	cmd.Flags().BoolVar(&f.noBase, "no-base", false, "skip base host roles")
	cmd.Flags().BoolVar(&f.rotateSecrets, "rotate-secrets", false, "force re-pull of secrets")
	cmd.Flags().BoolVar(&f.check, "check", false, "ansible --check (dry run)")
	cmd.Flags().StringVar(&f.overlay, "overlay", "", "extra inventory overlay file (relative to project)")
	cmd.Flags().BoolVar(&f.dryRun, "print", false, "print the docker invocation without running it")
	return cmd
}

func runDeploy(f *deployFlags) error {
	p, err := project.Load()
	if err != nil {
		return err
	}

	cmd := []string{
		"deploy.yml",
		"-i", filepath.Join(containerWorkspace, p.Config.Inventory),
	}
	if len(f.apps) > 0 {
		cmd = append(cmd, "--tags", strings.Join(f.apps, ","))
	}
	if len(f.servers) > 0 {
		cmd = append(cmd, "--limit", strings.Join(f.servers, ","))
	}
	if f.check {
		cmd = append(cmd, "--check")
	}
	if f.overlay != "" {
		cmd = append(cmd, "-i", filepath.Join(containerWorkspace, f.overlay))
	}
	switch {
	case f.base:
		cmd = append(cmd, "-e", "include_base=true")
	case f.noBase:
		cmd = append(cmd, "-e", "include_base=false")
	}
	if f.rotateSecrets {
		cmd = append(cmd, "-e", "rotate_secrets=true")
	}
	if globals.Verbose {
		cmd = append(cmd, "-v")
	}

	inv := baseInvocation(p)
	inv.Workdir = playbookDir
	inv.Entrypoint = "ansible-playbook"
	inv.Cmd = cmd

	if f.dryRun {
		fmt.Println("docker", strings.Join(inv.Args(), " "))
		return nil
	}
	if err := docker.Preflight(); err != nil {
		return err
	}
	return inv.Command().Run()
}
