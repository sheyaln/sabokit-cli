package cli

import (
	"os"

	"github.com/spf13/cobra"
)

const (
	DefaultRunnerImage = "ghcr.io/sheyaln/sabokit-runner"
	DefaultRunnerTag   = "v3.3.1"
	DefaultScwImage    = "scaleway/cli:2.56"
	DefaultTFImage     = "hashicorp/terraform:1.9"
)

type GlobalFlags struct {
	Image    string
	ScwImage string
	TFImage  string
	Platform string
	Env      string
	Verbose  bool
}

var globals GlobalFlags

func NewRootCmd() *cobra.Command {
	defaultImage := DefaultRunnerImage + ":" + DefaultRunnerTag
	if env := os.Getenv("SABOKIT_IMAGE"); env != "" {
		defaultImage = env
	}
	defaultPlatform := os.Getenv("SABOKIT_PLATFORM")
	defaultEnv := os.Getenv("SABOKIT_ENV")
	defaultScwImage := DefaultScwImage
	if env := os.Getenv("SABOKIT_SCW_IMAGE"); env != "" {
		defaultScwImage = env
	}
	defaultTFImage := DefaultTFImage
	if env := os.Getenv("SABOKIT_TF_IMAGE"); env != "" {
		defaultTFImage = env
	}

	cmd := &cobra.Command{
		Use:     "sabokit",
		Version: Version,
		Short:   "deploy and operate federated-commons stacks",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmd.SilenceUsage = true
		},
		Long: `sabokit orchestrates the sabokit-runner image to provision and manage
federated-commons stacks on scaleway. it is a thin orchestrator: terraform
and ansible run inside the runner image, sabokit shells out to docker.

requirements:
  docker (daemon must be live for deploy/down/status/up/secrets)
  ssh    (for ssh/logs and as ansible's transport)

a sabokit project is any directory containing .sabokit/config.yml. sabokit
walks up from cwd to find it. see README.md for the config schema.

env vars:
  SABOKIT_IMAGE                       overrides --image default (runner for ansible/terraform)
  SABOKIT_SCW_IMAGE                   overrides --scw-image default (scaleway cli for secrets ops)
  SABOKIT_TF_IMAGE                    overrides --tf-image default (terraform for up/destroy)
  SABOKIT_PLATFORM                    sets docker --platform (eg. linux/amd64 on arm64 hosts)
  SABOKIT_ENV                         overrides .sabokit/config.yml default_env

passed through to the runner image:
  SCW_ACCESS_KEY, SCW_SECRET_KEY, SCW_DEFAULT_PROJECT_ID,
  SCW_DEFAULT_ORGANIZATION_ID, SCW_DEFAULT_REGION, SCW_DEFAULT_ZONE`,
		Example: `  sabokit deploy --apps espocrm --check
  sabokit status --servers app01
  sabokit logs authentik -f
  sabokit apps list --enabled`,
		SilenceUsage:  false,
		SilenceErrors: false,
	}

	cmd.PersistentFlags().StringVar(&globals.Image, "image", defaultImage, "runner image ref for ansible/terraform (repository:tag); env: SABOKIT_IMAGE")
	cmd.PersistentFlags().StringVar(&globals.ScwImage, "scw-image", defaultScwImage, "scaleway cli image ref for secrets ops; env: SABOKIT_SCW_IMAGE")
	cmd.PersistentFlags().StringVar(&globals.TFImage, "tf-image", defaultTFImage, "terraform image ref for up/destroy; env: SABOKIT_TF_IMAGE")
	cmd.PersistentFlags().StringVar(&globals.Platform, "platform", defaultPlatform, "docker --platform override (eg. linux/amd64); env: SABOKIT_PLATFORM")
	cmd.PersistentFlags().StringVar(&globals.Env, "env", defaultEnv, "environment name under environments/<env>/; env: SABOKIT_ENV (overrides .sabokit/config.yml default_env)")
	cmd.PersistentFlags().BoolVarP(&globals.Verbose, "verbose", "v", false, "verbose output")

	cmd.AddCommand(
		newQuickstartCmd(),
		newInitCmd(),
		newConfigCmd(),
		newUpCmd(),
		newDeployCmd(),
		newDownCmd(),
		newDestroyCmd(),
		newStatusCmd(),
		newLogsCmd(),
		newSSHCmd(),
		newSecretsCmd(),
		newAppsCmd(),
		newEnvCmd(),
		newVersionCmd(),
	)

	return cmd
}
