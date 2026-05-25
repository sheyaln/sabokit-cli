package cli

import (
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <project-name>",
		Short: "scaffold a new consumer project",
		Long: `not yet implemented in v0.1.0.

planned behavior: render federated-commons' consumer-template/ into a new
directory named <project-name>. interactive prompts for org name, base
domain, scaleway region, ssh key path, etc. writes .sabokit/config.yml,
terraform.tfvars, inventory.ini, and apps-manifest.yaml.

template source is an open question: embed via //go:embed at build time,
or fetch from the federated-commons repo at a pinned tag at init time.

manual equivalent for v0.1.0: copy consumer-template/ from
federated-commons yourself and edit the files by hand.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("init")
		},
	}
}
