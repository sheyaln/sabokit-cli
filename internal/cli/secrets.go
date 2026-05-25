package cli

import (
	"github.com/spf13/cobra"
)

func newSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "manage scaleway secrets by human name",
		Long: `wraps 'scw secret' inside the runner image with a name-first surface that
hides scaleway's uuid-driven cli.

verified pain points the abstraction fixes:
  scw secret secret list  → 18-column wide uuid-dominant table; names buried
  scw secret secret get   → requires uuid (no get-by-name)
  scw secret version list / access / delete  → all require uuid
  scw secret version access-by-path  → returns base64-encoded data
  subcommand grammar  → 'scw secret secret list', 'scw secret version access'

sabokit's surface:
  every command takes the human name
  name → uuid resolution happens internally (one list call, cached)
  default version is 'latest'; --version N for older
  list is a tight 4-column table (NAME, TAGS, VERSIONS, UPDATED) by default
  get returns the plaintext value to stdout — already base64-decoded
  uuids only appear when --uuids is passed
  -o json mirrors scw's flag for scripting

all subcommands are stubs in v0.1.0. uses SCW_* env vars passed through to
the runner image.`,
	}
	cmd.AddCommand(
		newSecretsListCmd(),
		newSecretsGetCmd(),
		newSecretsVersionsCmd(),
		newSecretsCreateCmd(),
		newSecretsRotateCmd(),
		newSecretsDeleteCmd(),
	)
	return cmd
}

func newSecretsListCmd() *cobra.Command {
	var tag string
	var uuids bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list secrets by name (clean table, no uuids)",
		Long: `not yet implemented in v0.1.0.

planned behavior: 'scw secret secret list -o json' inside the runner image,
filter to a 4-column table (NAME, TAGS, VERSIONS, UPDATED). add ID as a
5th column when --uuids is set. --tag filters server-side via the scw
tags.{index} arg.

manual equivalent: 'scw secret secret list -o json | jq ...' and squint.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("secrets list")
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "filter by tag")
	cmd.Flags().BoolVar(&uuids, "uuids", false, "include the ID column in output")
	return cmd
}

func newSecretsGetCmd() *cobra.Command {
	var version string
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "print the plaintext value of a secret (default: latest version)",
		Long: `not yet implemented in v0.1.0.

planned behavior: resolve <name> → uuid (one scw call), then
'scw secret version access secret-id=<uuid> revision=<version> field=data',
base64-decode the result, write it to stdout with no trailing newline. no
headers, no field labels — pipe-friendly.

--version defaults to 'latest'. accepts numeric revisions too.

manual equivalent (the actual pain):
  uuid=$(scw secret secret list -o json | jq -r '.[]|select(.name=="X").id')
  scw secret version access secret-id=$uuid revision=latest -o json \
    | jq -r .data | base64 -d`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("secrets get")
		},
	}
	cmd.Flags().StringVar(&version, "version", "latest", "version (revision number or 'latest')")
	return cmd
}

func newSecretsVersionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "versions <name>",
		Short: "list all versions of a secret",
		Long: `not yet implemented in v0.1.0.

planned behavior: resolve <name> → uuid, then
'scw secret version list secret-id=<uuid>' formatted as
REVISION, STATUS, CREATED.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("secrets versions")
		},
	}
}

func newSecretsCreateCmd() *cobra.Command {
	var tag, description string
	cmd := &cobra.Command{
		Use:   "create <name> <value>",
		Short: "create a new secret with an initial value",
		Long: `not yet implemented in v0.1.0.

planned behavior: 'scw secret secret create name=<name>' (with optional
--tag and --description), then 'scw secret version create' with <value>
as version 1. fails if a secret with that name already exists — use
'sabokit secrets rotate' for new versions.

manual equivalent: 'scw secret secret create name=X' then
'scw secret version create secret-id=<uuid> data=<value>'.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("secrets create")
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "tag to attach to the secret (may repeat)")
	cmd.Flags().StringVar(&description, "description", "", "human description of the secret")
	return cmd
}

func newSecretsRotateCmd() *cobra.Command {
	var apps []string
	var fromStdin bool
	var fromFile string
	cmd := &cobra.Command{
		Use:   "rotate <name> [<value>]",
		Short: "push a new version of a secret and optionally redeploy affected apps",
		Long: `not yet implemented in v0.1.0.

planned behavior: resolve <name> → uuid, push a new version via
'scw secret version create'. value source (exactly one required):
  positional <value>      explicit on the command line
  --from-stdin            read value from stdin
  --from-file <path>      read value from a file

if --apps is set, chains into 'sabokit deploy --apps <apps> --rotate-secrets'
so the affected apps pick up the new value. if --apps is omitted, only the
version is pushed — you decide what to redeploy yourself.

manual equivalent: lookup uuid, 'scw secret version create
secret-id=<uuid> data=<value>', then 'sabokit deploy --apps <X>
--rotate-secrets'.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("secrets rotate")
		},
	}
	cmd.Flags().StringSliceVar(&apps, "apps", nil, "apps to redeploy after rotation")
	cmd.Flags().BoolVar(&fromStdin, "from-stdin", false, "read value from stdin")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "read value from a file")
	return cmd
}

func newSecretsDeleteCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "delete a secret (with confirmation prompt)",
		Long: `not yet implemented in v0.1.0.

planned behavior: resolve <name> → uuid, prompt for confirmation (showing
the name + tags + version count), then 'scw secret secret delete'. --yes
skips the prompt.

manual equivalent: lookup uuid, 'scw secret secret delete secret-id=<uuid>'.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("secrets delete")
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}
