package cli

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/spf13/cobra"
)

func newAppsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "manage the project's apps manifest",
		Long: `subcommands for inspecting and editing apps-manifest.yaml — the file that
tells sabokit which apps exist for the project and which host runs each.

"available" means listed in the manifest at all. "enabled" means
enabled: true. 'apps remove' toggles enabled to false; it does not delete
the entry.`,
	}
	cmd.AddCommand(newAppsListCmd(), newAppsAddCmd(), newAppsRemoveCmd())
	return cmd
}

func newAppsListCmd() *cobra.Command {
	var enabledOnly bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list apps in the project manifest (available + enabled)",
		Long: `tabular view of apps-manifest.yaml with columns NAME, ENABLED, HOST.
sorted by name. '-' in the HOST column means no host is set for that app.

this lists what's in the project's manifest. it does NOT enumerate the
universe of apps the runner image knows how to deploy — for that you'd
need to inspect the image's role catalog directly.`,
		Example: `  # all apps with their enabled status
  sabokit apps list

  # only currently-enabled apps
  sabokit apps list --enabled`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAppsList(enabledOnly)
		},
	}
	cmd.Flags().BoolVar(&enabledOnly, "enabled", false, "only show enabled apps")
	return cmd
}

func runAppsList(enabledOnly bool) error {
	p, err := project.Load()
	if err != nil {
		return err
	}
	apps, err := p.AllApps()
	if err != nil {
		return err
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].Name < apps[j].Name })

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tENABLED\tHOST")
	for _, a := range apps {
		if enabledOnly && !a.Enabled {
			continue
		}
		enabled := "no"
		if a.Enabled {
			enabled = "yes"
		}
		host := a.Host
		if host == "" {
			host = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", a.Name, enabled, host)
	}
	return tw.Flush()
}

func newAppsAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name>",
		Short: "add an app to apps-manifest.yaml and regenerate apps.tf",
		Long: `not yet implemented in v0.1.0.

planned behavior: append <name> to apps-manifest.yaml with enabled: true,
prompt for a host, then run scripts/gen_apps_yml.py inside the runner
image to regenerate apps.tf. prompts for terraform apply afterward.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("apps add")
		},
	}
}

func newAppsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "remove an app from apps-manifest.yaml",
		Long: `not yet implemented in v0.1.0.

planned behavior: toggle <name>'s enabled flag to false in
apps-manifest.yaml, prompt for 'sabokit down --apps <name>', then prompt
for terraform destroy of that app's resources.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("apps remove")
		},
	}
}
