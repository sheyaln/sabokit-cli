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
		Long: `runs ansible-playbook site.yml inside the runner image with the project
directory mounted at /workspace. site.yml is the umbrella (bootstrap + apps)
and lives in the image at /platform/ansible/site.yml — sabokit does not
ship it.

flag semantics:
  --apps         limits ansible --tags to the given app names
  --servers      limits ansible --limit to the given hosts
  --base         forces inclusion of base host roles (-e include_base=true)
  --no-base      skips base host roles (-e include_base=false)
                   --base and --no-base are mutually exclusive
  --rotate-secrets  passes -e rotate_secrets=true (forces secret re-pull)
  --check        ansible --check, no changes applied
  --overlay F    extra -i inventory file (path relative to project root)
  --print        print the docker invocation and exit (no docker required)

requires docker live (unless --print). default runner image is pinned via
--image; verbose mode (-v) maps to ansible -v.`,
		Example: `  # full deploy across all hosts
  sabokit deploy

  # deploy a single app to a single host, dry-run
  sabokit deploy --apps espocrm --servers app01 --check

  # rotate secrets and redeploy two apps
  sabokit deploy --apps espocrm,authentik --rotate-secrets

  # see the docker invocation without running it
  sabokit deploy --apps espocrm --print`,
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
		"site.yml",
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
