package cli

import (
	"github.com/spf13/cobra"
)

func newSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "manage scaleway secrets by human name",
		Long: `wraps 'scw secret' inside the runner image with a name-first surface.
scaleway's native cli is uuid-driven — listing returns uuids, reading a
value is a multi-step chain (find secret → find version → access). sabokit
hides that: every command takes the human name, resolves name → uuid
internally, defaults version selection to 'latest', and prints names (not
uuids) in output unless --uuids is passed.

not yet implemented in v0.1.0. uses SCW_* env vars passed through to the
runner image.`,
	}
	cmd.AddCommand(newSecretsCreateCmd(), newSecretsRotateCmd())
	return cmd
}

func newSecretsCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name> <value>",
		Short: "create a new scaleway secret with an initial value",
		Long: `not yet implemented in v0.1.0.

planned behavior: 'scw secret create name=<name>' inside the runner image,
then push <value> as version 1. fails if a secret with that name already
exists (use 'sabokit secrets rotate' for new versions).

manual equivalent: 'scw secret create name=<name>' then 'scw secret
version create secret-id=<uuid> data=<value>'.`,
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
		Short: "push a new version of a secret and redeploy affected apps",
		Long: `not yet implemented in v0.1.0.

planned behavior: resolve <name> → uuid, push a new version via
'scw secret version create', then chain into
'sabokit deploy --apps <apps> --rotate-secrets' so the affected apps pick
up the new value without a manual redeploy.

if --apps is omitted, no redeploy is run (you push the version, then
decide what to redeploy yourself).

manual equivalent: 'scw secret version create secret-id=<uuid> ...' then
'sabokit deploy --apps <X> --rotate-secrets'.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("secrets rotate")
		},
	}
	cmd.Flags().StringSliceVar(&apps, "apps", nil, "apps to redeploy after rotation")
	return cmd
}
