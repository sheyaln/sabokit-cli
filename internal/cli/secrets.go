package cli

import (
	"github.com/spf13/cobra"
)

func newSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "manage scaleway secrets",
		Long: `subcommands wrap 'scw secret' inside the runner image. uses your
SCW_* env vars (access key, secret key, project/org/region) — sabokit
passes them through to the container.

not yet implemented in v0.1.0.`,
	}
	cmd.AddCommand(newSecretsCreateCmd(), newSecretsRotateCmd())
	return cmd
}

func newSecretsCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name> <value>",
		Short: "create a new scaleway secret",
		Long: `not yet implemented in v0.1.0.

planned behavior: 'scw secret create name=<name>' inside the runner image,
then push <value> as the initial version. requires SCW_* env vars set.`,
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
		Long: `not yet implemented in v0.1.0.

planned behavior: push a new version of <name> via 'scw secret version
create', then chain into 'sabokit deploy --apps <apps> --rotate-secrets'
so the affected apps pick up the new value.

manual equivalent for v0.1.0: run 'scw secret version create' yourself,
then 'sabokit deploy --apps <X> --rotate-secrets'.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("secrets rotate")
		},
	}
	cmd.Flags().StringSliceVar(&apps, "apps", nil, "apps to redeploy after rotation")
	return cmd
}
