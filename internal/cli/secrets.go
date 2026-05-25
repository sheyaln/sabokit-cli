package cli

import (
	"github.com/spf13/cobra"
)

func newSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "manage scaleway secrets",
		Long: `deferred — may not ship at all. a thin proxy over 'scw secret' adds a
docker hop without value. only worth implementing if sabokit can replace
scaleway's uuid-driven surface with stable, human-readable names (resolve
name → uuid internally, hide uuids in output, support 'latest' and tagged
versions without lookups).

until then, use 'scw secret' directly inside the runner image (or on the
host if you have the scw cli installed).`,
	}
	cmd.AddCommand(newSecretsCreateCmd(), newSecretsRotateCmd())
	return cmd
}

func newSecretsCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name> <value>",
		Short: "create a new scaleway secret",
		Long: `deferred — see 'sabokit secrets --help' for the why.

manual: 'scw secret create name=<name>' then 'scw secret version create'
with the value.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("secrets create")
		},
	}
}

func newSecretsRotateCmd() *cobra.Command {
	var apps []string
	cmd := &cobra.Command{
		Use:   "rotate <name>",
		Short: "push new version of a secret and redeploy affected apps",
		Long: `deferred — see 'sabokit secrets --help' for the why.

manual: 'scw secret version create' then
'sabokit deploy --apps <X> --rotate-secrets'.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("secrets rotate")
		},
	}
	cmd.Flags().StringSliceVar(&apps, "apps", nil, "apps to redeploy after rotation")
	return cmd
}
