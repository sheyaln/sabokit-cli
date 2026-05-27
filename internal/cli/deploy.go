package cli

import (
	"encoding/json"
	"fmt"
	"os"
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
	rotateSecrets bool
	check         bool
	overlay       string
	dryRun        bool
}

func newDeployCmd() *cobra.Command {
	f := &deployFlags{}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "run ansible against the project (apps.yml by default; site.yml with --base)",
		Long: `runs an ansible playbook inside the runner image against the current env.
mounts the env dir at /workspace and uses inventory.ini from there.

playbook selection:
  default       /platform/ansible/apps.yml   — apps only, fast redeploy
  --base        /platform/ansible/site.yml   — bootstrap + apps (first-time)

env vars passed to ansible (auto, from the env dir):
  -e @/workspace/.ansible-vars.json (if present)
  -e env_name=<env>
  -e gateway_domain=<extracted from .ansible-vars.json's authentik_gateway_domain>

flag semantics:
  --apps         limits ansible --tags to the given app names
  --servers      limits ansible --limit to the given hosts
  --rotate-secrets   passes -e rotate_secrets=true (forces secret re-pull)
  --check        ansible --check, no changes applied
  --overlay F    extra -i inventory file (path inside the env dir)
  --print        print the docker invocation and exit (no docker required)`,
		Example: `  # apps-only redeploy of the env's default app set
  sabokit deploy

  # one app, one host, dry-run
  sabokit deploy --apps espocrm --servers app01 --check

  # first-time deploy (bootstrap + apps)
  sabokit deploy --base

  # secret rotation chain
  sabokit deploy --apps espocrm --rotate-secrets

  # see the docker invocation
  sabokit deploy --apps espocrm --print`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeploy(f)
		},
	}
	cmd.Flags().StringSliceVar(&f.apps, "apps", nil, "restrict to specific apps (ansible --tags)")
	cmd.Flags().StringSliceVar(&f.servers, "servers", nil, "restrict to specific hosts (ansible --limit)")
	cmd.Flags().BoolVar(&f.base, "base", false, "run site.yml (bootstrap + apps) instead of apps.yml")
	cmd.Flags().BoolVar(&f.rotateSecrets, "rotate-secrets", false, "force re-pull of secrets")
	cmd.Flags().BoolVar(&f.check, "check", false, "ansible --check (dry run)")
	cmd.Flags().StringVar(&f.overlay, "overlay", "", "extra inventory overlay file (relative to env dir)")
	cmd.Flags().BoolVar(&f.dryRun, "print", false, "print the docker invocation without running it")
	return cmd
}

func runDeploy(f *deployFlags) error {
	p, err := project.Load()
	if err != nil {
		return err
	}
	inv, err := buildDeployInvocation(p, f)
	if err != nil {
		return err
	}
	if f.dryRun {
		fmt.Println("docker", strings.Join(inv.Args(), " "))
		return nil
	}
	if err := docker.Preflight(); err != nil {
		return err
	}
	return inv.Command().Run()
}

func buildDeployInvocation(p *project.Project, f *deployFlags) (docker.Invocation, error) {
	playbook := "apps.yml"
	if f.base {
		playbook = "site.yml"
	}

	cmd := []string{
		playbook,
		"-i", containerWorkspace + "/" + p.Config.Inventory,
	}
	cmd = appendEnvExtraVars(cmd, p, globals.Env)
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
	if f.rotateSecrets {
		cmd = append(cmd, "-e", "rotate_secrets=true")
	}
	if globals.Verbose {
		cmd = append(cmd, "-v")
	}

	inv, err := baseInvocation(p)
	if err != nil {
		return docker.Invocation{}, err
	}
	inv.Workdir = playbookDir
	inv.Entrypoint = "ansible-playbook"
	inv.Cmd = cmd
	return inv, nil
}

// appendEnvExtraVars adds -e flags ansible playbooks expect when running
// against a consumer-template env: env_name, gateway_domain, and the
// .ansible-vars.json bundle. Skips silently if no env is set or the
// vars file doesn't exist yet (eg. before up.sh has run).
func appendEnvExtraVars(cmd []string, p *project.Project, envOverride string) []string {
	envName := p.EnvName(envOverride)
	if envName == "" {
		return cmd
	}
	cmd = append(cmd, "-e", "env_name="+envName)
	varsPath := p.AnsibleVarsPath(envOverride)
	if varsPath == "" {
		return cmd
	}
	if _, err := os.Stat(varsPath); err != nil {
		return cmd
	}
	cmd = append(cmd, "-e", "@"+containerWorkspace+"/.ansible-vars.json")
	if gateway, ok := readAnsibleVarGatewayDomain(varsPath); ok {
		cmd = append(cmd, "-e", "gateway_domain="+gateway)
	}
	return cmd
}

func readAnsibleVarGatewayDomain(path string) (string, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var v struct {
		AuthentikGatewayDomain string `json:"authentik_gateway_domain"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", false
	}
	if v.AuthentikGatewayDomain == "" {
		return "", false
	}
	return v.AuthentikGatewayDomain, true
}
