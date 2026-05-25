package cli

import (
	"github.com/spf13/cobra"
)

func newSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "manage scaleway secrets",
	}
	cmd.AddCommand(newSecretsCreateCmd(), newSecretsRotateCmd())
	return cmd
}

func newSecretsCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name> <value>",
		Short: "create a new scaleway secret",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("secrets create")
		},
	}
}

func newSecretsRotateCmd() *cobra.Command {
	var apps []string
	cmd := &cobra.Command{
		Use:   "rotate <name>",
		Short: "push new version of a secret and redeploy affected apps",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("secrets rotate")
		},
	}
	cmd.Flags().StringSliceVar(&apps, "apps", nil, "apps to redeploy after rotation")
	return cmd
}
