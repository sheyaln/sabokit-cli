package cli

import (
	"github.com/spf13/cobra"
)

func newUpCmd() *cobra.Command {
	var apps []string
	cmd := &cobra.Command{
		Use:   "up",
		Short: "full provision: terraform base+identity+apps, then ansible deploy",
		Long: `not yet implemented in v0.1.0.

planned behavior: 'terraform apply' against the base layer, then identity,
then apps (each via the runner image), and finally chain into
'sabokit deploy' across the resulting hosts. the happy-path for a first
deploy from scratch.

manual equivalent for v0.1.0: run terraform apply per layer yourself
inside the runner image, then 'sabokit deploy'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("up")
		},
	}
	cmd.Flags().StringSliceVar(&apps, "apps", nil, "restrict to specific apps")
	return cmd
}
