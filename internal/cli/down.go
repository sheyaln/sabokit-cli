package cli

import (
	"fmt"
	"strings"

	"github.com/sheyaln/sabokit-cli/internal/docker"
	"github.com/sheyaln/sabokit-cli/internal/envvalues"
	"github.com/sheyaln/sabokit-cli/internal/project"
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

to remove an app permanently, disable it in environments/<env>/
application.yml and run 'sabokit up --layers application' — terraform
destroys its resources declaratively.`,
		Example: `  sabokit down --apps espocrm
  sabokit down --apps espocrm,n8n --servers app01
  sabokit down --apps espocrm --print`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(apps) == 0 {
				return fmt.Errorf("--apps is required")
			}
			return runDownApps(apps, servers, dryRun, skipRefresh)
		},
	}
	cmd.Flags().StringSliceVar(&apps, "apps", nil, "apps to stop (required)")
	cmd.Flags().StringSliceVar(&servers, "servers", nil, "restrict to specific hosts")
	cmd.Flags().BoolVar(&dryRun, "print", false, "print the docker invocation without running it")
	cmd.Flags().BoolVar(&skipRefresh, "skip-refresh", false, "skip regenerating inventory.ini + .enabled_apps.json first")
	return cmd
}

func runDownApps(apps, servers []string, dryRun, skipRefresh bool) error {
	p, err := project.Load()
	if err != nil {
		return err
	}
	envName := p.EnvName(globals.Env)
	if envName == "" {
		return fmt.Errorf("down requires an env (pass --env or set default_env in .sabokit/config.yml)")
	}
	if err := requireCompatibleBlueprint(p); err != nil {
		return err
	}

	inv, err := downInvocation(p, envName, apps, servers)
	if err != nil {
		return err
	}
	if dryRun {
		fmt.Println("docker", strings.Join(inv.Args(), " "))
		return nil
	}
	if err := docker.Preflight(); err != nil {
		return err
	}
	if !skipRefresh {
		if err := runScript(p, "refresh.sh", envName); err != nil {
			return err
		}
	}
	return inv.Command().Run()
}

func downInvocation(p *project.Project, envName string, apps, servers []string) (docker.Invocation, error) {
	envRel := "environments/" + envName
	cmd := []string{
		playbookDir + "/down.yml",
		"-i", envRel + "/inventory.ini",
		"-e", "@" + envRel + "/.enabled_apps.json",
		"-e", "env_name=" + envName,
	}
	if values, err := envvalues.ForEnv(p.Root, envName); err == nil {
		if gw := values.String("identity_domain"); gw != "" {
			cmd = append(cmd, "-e", "identity_domain="+gw)
		}
	}
	cmd = append(cmd, "--tags", strings.Join(apps, ","))
	if len(servers) > 0 {
		cmd = append(cmd, "--limit", strings.Join(servers, ","))
	}
	if globals.Verbose {
		cmd = append(cmd, "-v")
	}

	inv, err := repoInvocation(p)
	if err != nil {
		return docker.Invocation{}, err
	}
	inv.Entrypoint = "ansible-playbook"
	inv.Cmd = cmd
	// Plays run with the consumer repo as cwd (no ansible.cfg there) —
	// mirror lib.sh's exports so role resolution doesn't depend on cwd.
	inv.Env["ANSIBLE_CONFIG"] = playbookDir + "/ansible.cfg"
	inv.Env["ANSIBLE_ROLES_PATH"] = "/opt/sabokit/platform/infra/ansible/roles:/opt/sabokit/platform/identity/ansible/roles:" + containerRepo + "/ansible-local/roles"
	inv.Env["ANSIBLE_HOST_KEY_CHECKING"] = "False"
	return inv, nil
}
