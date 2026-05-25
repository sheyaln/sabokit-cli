package cli

import (
	"github.com/spf13/cobra"
)

func newAppsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "manage the project's apps manifest",
	}
	cmd.AddCommand(newAppsAddCmd(), newAppsRemoveCmd())
	return cmd
}

func newAppsAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name>",
		Short: "add an app to apps-manifest.yaml and regenerate apps.tf",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("apps add")
		},
	}
}

func newAppsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "remove an app from apps-manifest.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("apps remove")
		},
	}
}
