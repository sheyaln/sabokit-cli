package cli

import (
	"fmt"

	"github.com/sheyaln/sabokit-cli/internal/docker"
	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/spf13/cobra"
)

func newRefreshCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "regenerate inventory.ini + .enabled_apps.json from terraform state",
		Long: `runs scripts/refresh.sh inside the runner image: rebuilds the env's
inventory.ini and .enabled_apps.json from the layers' current terraform
state. the layer scripts run this themselves before every ansible play;
run it manually after an out-of-band terraform change.`,
		Example: `  sabokit refresh
  sabokit --env staging refresh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := project.Load()
			if err != nil {
				return err
			}
			envName := p.EnvName(globals.Env)
			if envName == "" {
				return fmt.Errorf("refresh requires an env (pass --env or set default_env in .sabokit/config.yml)")
			}
			if err := requireCompatibleBlueprint(p); err != nil {
				return err
			}
			if err := docker.Preflight(); err != nil {
				return err
			}
			return runScript(p, "refresh.sh", envName)
		},
	}
}
