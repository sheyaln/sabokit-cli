package cli

import (
	"os"
	"runtime"

	"github.com/sheyaln/sabokit-cli/internal/version"
	"github.com/spf13/cobra"
)

const (
	DefaultRunnerImage = "ghcr.io/sheyaln/sabokit-runner"
	DefaultScwImage    = "scaleway/cli:2.56"
	DefaultTFImage     = "hashicorp/terraform:1.13"
)

type GlobalFlags struct {
	Image            string
	ScwImage         string
	TFImage          string
	Platform         string
	Env              string
	Verbose          bool
	SkipVersionCheck bool
}

var globals GlobalFlags

func NewRootCmd() *cobra.Command {
	// Empty default => baseInvocation resolves the runner image tag from the
	// environment's pinned sabokit version, so the ansible half matches the
	// terraform half. SABOKIT_IMAGE / --image override.
	defaultImage := os.Getenv("SABOKIT_IMAGE")
	defaultPlatform := os.Getenv("SABOKIT_PLATFORM")
	if defaultPlatform == "" {
		// Pin the host's native arch so docker pulls the matching variant from
		// each image's multi-arch manifest, and never silently emulates (eg.
		// when DOCKER_DEFAULT_PLATFORM is set). terraform, scaleway/cli, and
		// sabokit-runner all ship linux/amd64 + linux/arm64.
		defaultPlatform = "linux/" + runtime.GOARCH
	}
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
		Version: version.CLI,
		Short:   "deploy and operate sabokit stacks",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmd.SilenceUsage = true
		},
		Long: `sabokit orchestrates the sabokit-runner image to provision and manage
sabokit stacks on scaleway. it is a thin orchestrator: terraform
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
  SABOKIT_PLATFORM                    docker --platform override (defaults to host arch; all images are multi-arch)
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

	cmd.PersistentFlags().StringVar(&globals.Image, "image", defaultImage, "runner image ref for ansible (repository:tag); default: sabokit-runner at the env's pinned version; env: SABOKIT_IMAGE")
	cmd.PersistentFlags().StringVar(&globals.ScwImage, "scw-image", defaultScwImage, "scaleway cli image ref for secrets ops; env: SABOKIT_SCW_IMAGE")
	cmd.PersistentFlags().StringVar(&globals.TFImage, "tf-image", defaultTFImage, "terraform image ref for up/destroy; env: SABOKIT_TF_IMAGE")
	cmd.PersistentFlags().StringVar(&globals.Platform, "platform", defaultPlatform, "docker --platform override; defaults to host arch (eg. force emulation with linux/amd64); env: SABOKIT_PLATFORM")
	cmd.PersistentFlags().StringVar(&globals.Env, "env", defaultEnv, "environment name under environments/<env>/; env: SABOKIT_ENV (overrides .sabokit/config.yml default_env)")
	cmd.PersistentFlags().BoolVar(&globals.SkipVersionCheck, "skip-version-check", false, "skip the CLI⇄blueprint compatibility check (unsafe: mismatched terraform/ansible can corrupt state)")
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
