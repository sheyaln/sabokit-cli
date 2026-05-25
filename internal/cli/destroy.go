package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDestroyCmd() *cobra.Command {
	var apps []string
	var layer string
	var all bool
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "terraform destroy (per-app, per-layer, or full)",
		RunE: func(cmd *cobra.Command, args []string) error {
			modes := 0
			if len(apps) > 0 {
				modes++
			}
			if layer != "" {
				modes++
			}
			if all {
				modes++
			}
			if modes != 1 {
				return fmt.Errorf("exactly one of --apps, --layer, --all required")
			}
			if layer != "" {
				switch layer {
				case "base", "identity", "apps":
				default:
					return fmt.Errorf("--layer must be one of: base, identity, apps")
				}
			}
			return notImplemented("destroy")
		},
	}
	cmd.Flags().StringSliceVar(&apps, "apps", nil, "destroy specific apps (terraform -target)")
	cmd.Flags().StringVar(&layer, "layer", "", "destroy whole layer: base | identity | apps")
	cmd.Flags().BoolVar(&all, "all", false, "destroy everything (apps → identity → base)")
	return cmd
}
