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
		Use:           "sabokit",
		Short:         "deploy and operate federated-commons stacks",
		Long:          "sabokit orchestrates the sabokit-runner image to provision and manage federated-commons stacks on scaleway.",
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	cmd.PersistentFlags().StringVar(&globals.Image, "image", DefaultRunnerImage+":"+DefaultRunnerTag, "runner image ref (repository:tag)")
	cmd.PersistentFlags().BoolVarP(&globals.Verbose, "verbose", "v", false, "verbose output")

	cmd.AddCommand(
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
