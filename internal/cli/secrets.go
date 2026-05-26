package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/sheyaln/sabokit-cli/internal/docker"
	"github.com/sheyaln/sabokit-cli/internal/scw"
	"github.com/spf13/cobra"
)

func newSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "manage scaleway secrets by human name",
		Long: `wraps 'scw secret' inside the runner image with a name-first surface that
hides scaleway's uuid-driven cli.

sabokit's surface:
  every command takes the human name
  name → uuid resolution happens internally (one list call, cached)
  default version is 'latest'; --version N for older
  list is a tight 4-column table (NAME, TAGS, VERSIONS, UPDATED) by default
  get returns the plaintext value to stdout — already base64-decoded
  uuids only appear when --uuids is passed
  versions is sorted newest-first by revision number

uses SCW_* env vars passed through to the scaleway cli image (default:
scaleway/cli:2.56, override via --scw-image or SABOKIT_SCW_IMAGE). requires
docker live. does not require a sabokit project (.sabokit/config.yml).`,
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

func newScwClient() (*scw.Client, error) {
	if err := docker.Preflight(); err != nil {
		return nil, err
	}
	return scw.New(globals.ScwImage, globals.Platform), nil
}

func newSecretsListCmd() *cobra.Command {
	var tag string
	var uuids bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list secrets by name (clean table, no uuids)",
		Long: `clean 4-column table sorted by name: NAME, TAGS, VERSIONS, UPDATED.
add the ID column with --uuids. --tag filters client-side.`,
		Example: `  sabokit secrets list
  sabokit secrets list --tag authentik
  sabokit secrets list --uuids`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSecretsList(tag, uuids)
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "filter by tag")
	cmd.Flags().BoolVar(&uuids, "uuids", false, "include the ID column")
	return cmd
}

func runSecretsList(tag string, uuids bool) error {
	c, err := newScwClient()
	if err != nil {
		return err
	}
	secrets, err := c.ListSecrets()
	if err != nil {
		return err
	}
	if tag != "" {
		secrets = filterByTag(secrets, tag)
	}
	sort.Slice(secrets, func(i, j int) bool { return secrets[i].Name < secrets[j].Name })

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if uuids {
		fmt.Fprintln(tw, "NAME\tTAGS\tVERSIONS\tUPDATED\tID")
	} else {
		fmt.Fprintln(tw, "NAME\tTAGS\tVERSIONS\tUPDATED")
	}
	for _, s := range secrets {
		tags := strings.Join(s.Tags, ",")
		if tags == "" {
			tags = "-"
		}
		updated := shortTime(s.UpdatedAt)
		if uuids {
			fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n", s.Name, tags, s.VersionCount, updated, s.ID)
		} else {
			fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", s.Name, tags, s.VersionCount, updated)
		}
	}
	return tw.Flush()
}

func filterByTag(secrets []scw.Secret, tag string) []scw.Secret {
	out := secrets[:0:0]
	for _, s := range secrets {
		for _, t := range s.Tags {
			if t == tag {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

func shortTime(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func newSecretsGetCmd() *cobra.Command {
	var version string
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "print the plaintext value of a secret (default: latest version)",
		Long: `resolves <name> → uuid, fetches the version, base64-decodes the data,
writes it to stdout with no trailing newline. pipe-friendly.`,
		Example: `  sabokit secrets get authentik-admin-password
  sabokit secrets get authentik-admin-password --version 2
  sabokit secrets get db-url > /tmp/db.url`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSecretsGet(args[0], version)
		},
	}
	cmd.Flags().StringVar(&version, "version", "latest", "version (revision number or 'latest')")
	return cmd
}

func runSecretsGet(name, version string) error {
	rev, err := scw.ParseRevision(version)
	if err != nil {
		return err
	}
	c, err := newScwClient()
	if err != nil {
		return err
	}
	s, err := c.Resolve(name)
	if err != nil {
		return err
	}
	data, err := c.AccessVersion(s.ID, rev)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}

func newSecretsVersionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "versions <name>",
		Short: "list all versions of a secret",
		Long: `resolves <name> → uuid and lists revisions with status and created date.`,
		Example: `  sabokit secrets versions authentik-admin-password`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSecretsVersions(args[0])
		},
	}
}

func runSecretsVersions(name string) error {
	c, err := newScwClient()
	if err != nil {
		return err
	}
	s, err := c.Resolve(name)
	if err != nil {
		return err
	}
	versions, err := c.ListVersions(s.ID)
	if err != nil {
		return err
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i].Revision > versions[j].Revision })

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "REVISION\tSTATUS\tCREATED")
	for _, v := range versions {
		fmt.Fprintf(tw, "%d\t%s\t%s\n", v.Revision, v.Status, shortTime(v.CreatedAt))
	}
	return tw.Flush()
}

func newSecretsCreateCmd() *cobra.Command {
	var tags []string
	var description string
	cmd := &cobra.Command{
		Use:   "create <name> <value>",
		Short: "create a new secret with an initial value",
		Long: `creates the secret and pushes <value> as version 1 in one call. fails if
a secret with that name already exists.`,
		Example: `  sabokit secrets create db-password 'hunter2'
  sabokit secrets create api-key XYZ --tag espocrm --description 'EspoCRM API key'`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSecretsCreate(args[0], args[1], tags, description)
		},
	}
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "tag to attach (may repeat)")
	cmd.Flags().StringVar(&description, "description", "", "human description")
	return cmd
}

func runSecretsCreate(name, value string, tags []string, description string) error {
	c, err := newScwClient()
	if err != nil {
		return err
	}
	if _, err := c.Resolve(name); err == nil {
		return fmt.Errorf("secret %q already exists — use 'sabokit secrets rotate' to push a new version", name)
	}
	s, err := c.CreateSecret(name, tags, description)
	if err != nil {
		return err
	}
	rev, err := c.PushVersion(s.ID, []byte(value))
	if err != nil {
		return fmt.Errorf("created secret %q but failed to push initial version: %w", name, err)
	}
	fmt.Printf("created %s (v%d)\n", name, rev)
	return nil
}

func newSecretsRotateCmd() *cobra.Command {
	var apps []string
	var fromStdin bool
	var fromFile string
	cmd := &cobra.Command{
		Use:   "rotate <name> [<value>]",
		Short: "push a new version of a secret and optionally redeploy affected apps",
		Long: `pushes a new version of <name>. value source (exactly one required):
  positional <value>      explicit on the command line
  --from-stdin            read value from stdin
  --from-file <path>      read value from a file

if --apps is set, chains into 'sabokit deploy --apps <apps> --rotate-secrets'
so affected apps pick up the new value.`,
		Example: `  sabokit secrets rotate db-password 'new-pass'
  echo 'new-pass' | sabokit secrets rotate db-password --from-stdin
  sabokit secrets rotate api-key --from-file /tmp/key.txt --apps espocrm`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			var value string
			if len(args) == 2 {
				value = args[1]
			}
			return runSecretsRotate(name, value, fromStdin, fromFile, apps)
		},
	}
	cmd.Flags().StringSliceVar(&apps, "apps", nil, "apps to redeploy after rotation (--rotate-secrets)")
	cmd.Flags().BoolVar(&fromStdin, "from-stdin", false, "read value from stdin")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "read value from a file")
	return cmd
}

func runSecretsRotate(name, value string, fromStdin bool, fromFile string, apps []string) error {
	data, err := pickValueSource(value, fromStdin, fromFile)
	if err != nil {
		return err
	}
	c, err := newScwClient()
	if err != nil {
		return err
	}
	s, err := c.Resolve(name)
	if err != nil {
		return err
	}
	rev, err := c.PushVersion(s.ID, data)
	if err != nil {
		return err
	}
	fmt.Printf("rotated %s → v%d\n", name, rev)

	if len(apps) > 0 {
		fmt.Printf("redeploying: %s\n", strings.Join(apps, ","))
		return runDeploy(&deployFlags{apps: apps, rotateSecrets: true})
	}
	return nil
}

func pickValueSource(value string, fromStdin bool, fromFile string) ([]byte, error) {
	sources := 0
	if value != "" {
		sources++
	}
	if fromStdin {
		sources++
	}
	if fromFile != "" {
		sources++
	}
	if sources == 0 {
		return nil, fmt.Errorf("a value is required: pass <value>, --from-stdin, or --from-file")
	}
	if sources > 1 {
		return nil, fmt.Errorf("only one of <value>, --from-stdin, --from-file may be used")
	}
	switch {
	case fromStdin:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		return bytesNoTrailingNewline(data), nil
	case fromFile != "":
		data, err := os.ReadFile(fromFile)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", fromFile, err)
		}
		return bytesNoTrailingNewline(data), nil
	default:
		return []byte(value), nil
	}
}

func bytesNoTrailingNewline(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}

func newSecretsDeleteCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "delete a secret (with confirmation prompt)",
		Long: `resolves <name> → uuid, shows the secret's metadata, then prompts for
confirmation. --yes skips the prompt.`,
		Example: `  sabokit secrets delete old-key
  sabokit secrets delete old-key --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSecretsDelete(args[0], yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func runSecretsDelete(name string, yes bool) error {
	c, err := newScwClient()
	if err != nil {
		return err
	}
	s, err := c.Resolve(name)
	if err != nil {
		return err
	}
	if !yes {
		fmt.Printf("about to delete: %s\n  id:       %s\n  tags:     %s\n  versions: %d\nproceed? [y/N] ", s.Name, s.ID, strings.Join(s.Tags, ","), s.VersionCount)
		r := bufio.NewReader(os.Stdin)
		line, _ := r.ReadString('\n')
		line = strings.ToLower(strings.TrimSpace(line))
		if line != "y" && line != "yes" {
			return fmt.Errorf("aborted")
		}
	}
	if err := c.DeleteSecret(s.ID); err != nil {
		return err
	}
	fmt.Printf("deleted %s\n", name)
	return nil
}
