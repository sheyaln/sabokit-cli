package cli

import (
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <project-name>",
		Short: "scaffold a new consumer project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("init")
		},
	}
}
