package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "0.1.0-dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print binary version and default runner image",
		Long: `prints two lines:
  sabokit <semver>            — binary version, injected via ldflag at build time
  runner  <repo>:<tag>        — default runner image used unless --image overrides

dev builds report 'sabokit <next-version>-dev'. release builds report the
tag that triggered the workflow (eg. 'sabokit 0.1.0'). versions are
semver — vX.Y.Z, released in tandem with the sabokit blueprint.`,
		Example: `  sabokit version`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("sabokit %s\nrunner %s:%s\n", Version, DefaultRunnerImage, DefaultRunnerTag)
			return nil
		},
	}
}
