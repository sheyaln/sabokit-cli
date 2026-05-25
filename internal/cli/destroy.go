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
		Long: `not yet implemented in v0.1.0 — destroy is a manual terraform operation
for now.

planned behavior:
  --apps X[,Y]   terraform destroy -target=module.<app> for each named app
  --layer L      destroy a whole layer (base | identity | apps)
  --all          destroy everything, apps → identity → base, with prompt

exactly one of --apps, --layer, --all is required.

manual equivalent for v0.1.0: run terraform destroy directly against the
relevant layer inside the runner image.`,
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
