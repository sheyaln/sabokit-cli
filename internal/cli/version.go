package cli

import (
	"fmt"

	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/sheyaln/sabokit-cli/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print the CLI version and the blueprint range it supports",
		Long: `prints the CLI's own semver and the range of sabokit blueprint
versions it can drive:

  sabokit    <semver>     — this binary's version (ldflag-injected at build)
  supports   <range>      — blueprint major.minor lines this CLI can operate

the CLI does not pick a blueprint version — each environment does, via the
?ref= its terraform pins. run inside a project, this also prints every env's
pinned sabokit version and whether it falls in the supported range. an env
outside the range is refused (override: --skip-version-check).`,
		Example: `  sabokit version`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("sabokit    %s\nsupports   %s   (runner %s)\n",
				version.CLI, version.SupportedRange(), DefaultRunnerImage)
			printEnvPins()
			return nil
		},
	}
}

// printEnvPins lists each env's pinned sabokit version + compat, when run
// inside a project. Silent (best-effort) outside a project.
func printEnvPins() {
	p, err := project.Load()
	if err != nil {
		return
	}
	envs := p.Envs()
	if len(envs) == 0 {
		return
	}
	fmt.Println("environments:")
	for _, env := range envs {
		ref, err := p.BlueprintVersion(env)
		if err != nil {
			fmt.Printf("  %-10s ?  (%v)\n", env, err)
			continue
		}
		mark := "ok"
		if ok, _ := version.Supports(ref); !ok {
			mark = "UNSUPPORTED"
		}
		fmt.Printf("  %-10s %s  [%s]\n", env, ref, mark)
	}
}
