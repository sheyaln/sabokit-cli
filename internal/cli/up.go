package cli

import (
	"github.com/spf13/cobra"
)

func newUpCmd() *cobra.Command {
	var apps []string
	cmd := &cobra.Command{
		Use:   "up",
		Short: "full provision: terraform base+identity+apps, then ansible deploy",
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("up")
		},
	}
	cmd.Flags().StringSliceVar(&apps, "apps", nil, "restrict to specific apps")
	return cmd
}
