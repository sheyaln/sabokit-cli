package cli

import (
	"github.com/spf13/cobra"
)

const (
	DefaultRunnerImage = "ghcr.io/sheyaln/sabokit-runner"
	DefaultRunnerTag   = "v3.0.0"
)

type GlobalFlags struct {
	Image   string
	Verbose bool
}

var globals GlobalFlags

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sabokit",
		Short: "deploy and operate federated-commons stacks",
		Long: `sabokit orchestrates the sabokit-runner image to provision and manage
federated-commons stacks on scaleway. it is a thin orchestrator: terraform
and ansible run inside the runner image, sabokit shells out to docker.

requirements:
  docker (daemon must be live for deploy/down/status/up/secrets)
  ssh    (for ssh/logs and as ansible's transport)

a sabokit project is any directory containing .sabokit/config.yml. sabokit
walks up from cwd to find it. see README.md for the config schema.

env vars passed through to the runner image:
  SCW_ACCESS_KEY, SCW_SECRET_KEY, SCW_DEFAULT_PROJECT_ID,
  SCW_DEFAULT_ORGANIZATION_ID, SCW_DEFAULT_REGION, SCW_DEFAULT_ZONE`,
		Example: `  sabokit deploy --apps espocrm --check
  sabokit status --servers app01
  sabokit logs authentik -f
  sabokit apps list --enabled`,
		SilenceUsage:  false,
		SilenceErrors: false,
	}

	cmd.PersistentFlags().StringVar(&globals.Image, "image", DefaultRunnerImage+":"+DefaultRunnerTag, "runner image ref (repository:tag)")
	cmd.PersistentFlags().BoolVarP(&globals.Verbose, "verbose", "v", false, "verbose output")

	cmd.AddCommand(
		newQuickstartCmd(),
		newInitCmd(),
		newUpCmd(),
		newDeployCmd(),
		newDownCmd(),
		newDestroyCmd(),
		newStatusCmd(),
		newLogsCmd(),
		newSSHCmd(),
		newSecretsCmd(),
		newAppsCmd(),
		newVersionCmd(),
	)

	return cmd
}
