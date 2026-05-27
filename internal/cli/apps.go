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
		Short: "manage the project's apps catalog and per-env enabled state",
		Long: `subcommands for inspecting and editing the apps catalog (apps-manifest.yaml
at project root) and the per-env enabled state (env's .ansible-vars.json).

apps-manifest.yaml is producer-curated — the upstream catalog of every app
sabokit can deploy. It is NOT where you set per-env enabled state. That
lives in environments/<env>/config.tf and gets propagated to
environments/<env>/.ansible-vars.json by up.sh.`,
	}
	cmd.AddCommand(newAppsListCmd(), newAppsAddCmd(), newAppsRemoveCmd())
	return cmd
}

func newAppsListCmd() *cobra.Command {
	var enabled bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list available apps (catalog) or env-enabled apps with --enabled",
		Long: `two modes:

  default        NAME, CATEGORY, DESCRIPTION
                 reads apps-manifest.yaml at project root.

  --enabled      NAME, URL
                 reads .ansible-vars.json from environments/<env>/.
                 only apps with a non-null entry are shown — those are
                 the ones TF actually provisioned. requires an env.`,
		Example: `  sabokit apps list                # catalog (always available)
  sabokit apps list --enabled      # what's running in the current env`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAppsList(enabled)
		},
	}
	cmd.Flags().BoolVar(&enabled, "enabled", false, "show env-resolved enabled apps instead of the catalog")
	return cmd
}

func runAppsList(enabled bool) error {
	p, err := project.Load()
	if err != nil {
		return err
	}
	if enabled {
		if p.EnvName(globals.Env) == "" {
			return fmt.Errorf("--enabled requires an env (pass --env or set default_env in .sabokit/config.yml)")
		}
		return printEnabledApps(p)
	}
	return printCatalog(p)
}

func printCatalog(p *project.Project) error {
	apps, err := p.Catalog()
	if err != nil {
		return err
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].ID < apps[j].ID })
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tCATEGORY\tDESCRIPTION")
	for _, a := range apps {
		desc := a.DescriptionShort
		if desc == "" {
			desc = "-"
		}
		cat := a.Category
		if cat == "" {
			cat = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", a.ID, cat, desc)
	}
	return tw.Flush()
}

func printEnabledApps(p *project.Project) error {
	apps, err := p.EnvApps(globals.Env)
	if err != nil {
		return err
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].ID < apps[j].ID })
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tURL")
	for _, a := range apps {
		if !a.Enabled {
			continue
		}
		fmt.Fprintf(tw, "%s\t%s\n", a.ID, a.URL)
	}
	return tw.Flush()
}

func newAppsAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name>",
		Short: "add an app — not yet implemented",
		Long: `not yet implemented in v0.1.x.

apps are enabled per-env by editing environments/<env>/config.tf's
locals.config.apps block. there is no project-root catalog edit; the
catalog is producer-curated upstream.

manual equivalent: edit config.tf, set apps.<name>.enabled = true, then
re-run 'sabokit deploy --base' or the env's configure.sh.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("apps add")
		},
	}
}

func newAppsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "remove an app — not yet implemented",
		Long: `not yet implemented in v0.1.x.

manual equivalent: edit environments/<env>/config.tf, set
apps.<name>.enabled = false, then 'sabokit down --apps <name>' and
'sabokit deploy --base' (or re-run configure.sh).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("apps remove")
		},
	}
}
