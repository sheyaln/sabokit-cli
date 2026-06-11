package cli

import (
	"fmt"
	"strings"

	"github.com/sheyaln/sabokit-cli/internal/docker"
	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/spf13/cobra"
)

type deployFlags struct {
	apps          []string
	servers       []string
	all           bool
	rotateSecrets bool
	check         bool
	dryRun        bool
}

func newDeployCmd() *cobra.Command {
	f := &deployFlags{}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "ansible-only redeploy (no terraform) via scripts/deploy.sh",
		Long: `runs scripts/deploy.sh inside the runner image against the current env:
regenerates inventory.ini + .enabled_apps.json from the layers' terraform
state, then runs the site playbook with the selected tags.

tag selection:
  default       application       — every enabled app (fast redeploy)
  --apps X,Y    X,Y               — just those apps
  --all         all               — bootstrap + host-services + ops + apps

no terraform runs here. when terraform-managed objects changed too (a new
app enabled, a hostname change, secrets), run the owning layer instead:
'sabokit up --layers application'.`,
		Example: `  sabokit deploy                        # all enabled apps
  sabokit deploy --apps espocrm         # one app
  sabokit deploy --apps espocrm --check # dry run
  sabokit deploy --all                  # bootstrap + everything
  sabokit deploy --apps espocrm --rotate-secrets`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeploy(f)
		},
	}
	cmd.Flags().StringSliceVar(&f.apps, "apps", nil, "restrict to specific apps (ansible --tags)")
	cmd.Flags().StringSliceVar(&f.servers, "servers", nil, "restrict to specific hosts (ansible --limit)")
	cmd.Flags().BoolVar(&f.all, "all", false, "run every tag (bootstrap + host-services + operations + application)")
	cmd.Flags().BoolVar(&f.rotateSecrets, "rotate-secrets", false, "force re-pull of secrets")
	cmd.Flags().BoolVar(&f.check, "check", false, "ansible --check (dry run)")
	cmd.Flags().BoolVar(&f.dryRun, "print", false, "print the docker invocation without running it")
	return cmd
}

func runDeploy(f *deployFlags) error {
	if f.all && len(f.apps) > 0 {
		return fmt.Errorf("--all and --apps are mutually exclusive")
	}
	p, err := project.Load()
	if err != nil {
		return err
	}
	envName := p.EnvName(globals.Env)
	if envName == "" {
		return fmt.Errorf("deploy requires an env (pass --env or set default_env in .sabokit/config.yml)")
	}
	if err := requireCompatibleBlueprint(p); err != nil {
		return err
	}

	tags := "application"
	switch {
	case f.all:
		tags = "all"
	case len(f.apps) > 0:
		tags = strings.Join(f.apps, ",")
	}
	args := []string{envName, tags}
	if len(f.servers) > 0 {
		args = append(args, "--limit", strings.Join(f.servers, ","))
	}
	if f.check {
		args = append(args, "--check")
	}
	if f.rotateSecrets {
		args = append(args, "-e", "rotate_secrets=true")
	}
	if globals.Verbose {
		args = append(args, "-v")
	}

	if f.dryRun {
		inv, err := scriptInvocation(p, "deploy.sh", args...)
		if err != nil {
			return err
		}
		fmt.Println("docker", strings.Join(inv.Args(), " "))
		return nil
	}
	if err := docker.Preflight(); err != nil {
		return err
	}
	return runScript(p, "deploy.sh", args...)
}
