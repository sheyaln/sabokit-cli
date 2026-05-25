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
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("sabokit %s\nrunner %s:%s\n", Version, DefaultRunnerImage, DefaultRunnerTag)
			return nil
		},
	}
}
