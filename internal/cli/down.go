package cli

import (
	"fmt"
	"strings"

	"github.com/sheyaln/sabokit-cli/internal/docker"
	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/sheyaln/sabokit-cli/internal/tf"
	"github.com/spf13/cobra"
)

func newDownCmd() *cobra.Command {
	var apps, servers []string
	var dryRun, skipRefresh bool
	cmd := &cobra.Command{
		Use:   "down",
		Short: "stop apps (docker compose down) without destroying cloud resources",
		Long: `runs ansible-playbook down.yml --tags <apps> inside the runner image
against the current env. each named app's containers are stopped via
docker compose down. cloud resources (instances, dns, secrets, TF state)
are untouched. reversible via 'sabokit deploy --apps <same>'.

passes the same env-aware -e flags as deploy (env_name,
@/workspace/.ansible-vars.json, gateway_domain).

runs against the sabokit-runner image (defaults to the version in
'sabokit version'); down.yml lives at /opt/sabokit/platform/ansible/.`,
		Example: `  sabokit down --apps espocrm
  sabokit down --apps espocrm,n8n --servers app01
  sabokit down --apps espocrm --print`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(apps) == 0 {
				return fmt.Errorf("--apps is required")
			}
			return runDown(apps, servers, dryRun, skipRefresh)
		},
	}
	cmd.Flags().StringSliceVar(&apps, "apps", nil, "apps to stop (required)")
	cmd.Flags().StringSliceVar(&servers, "servers", nil, "restrict to specific hosts")
	cmd.Flags().BoolVar(&dryRun, "print", false, "print the docker invocation without running it")
	cmd.Flags().BoolVar(&skipRefresh, "skip-refresh", false, "skip re-running terraform output to rebuild inventory.ini + .ansible-vars.json")
	return cmd
}

func runDown(apps, servers []string, dryRun, skipRefresh bool) error {
	p, err := project.Load()
	if err != nil {
		return err
	}
	if !dryRun && !skipRefresh {
		if err := docker.Preflight(); err != nil {
			return err
		}
		if err := refreshIfEnv(p, tf.New(globals.TFImage, globals.Platform, p.Root)); err != nil {
			return err
		}
	}

	cmd := []string{
		"down.yml",
		"-i", containerWorkspace + "/" + p.Config.Inventory,
	}
	cmd = appendEnvExtraVars(cmd, p, globals.Env)
	cmd = append(cmd, "--tags", strings.Join(apps, ","))
	if len(servers) > 0 {
		cmd = append(cmd, "--limit", strings.Join(servers, ","))
	}
	if globals.Verbose {
		cmd = append(cmd, "-v")
	}

	inv, err := baseInvocation(p)
	if err != nil {
		return err
	}
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
