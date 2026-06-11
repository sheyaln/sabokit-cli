package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/sheyaln/sabokit-cli/internal/appsyaml"
	"github.com/sheyaln/sabokit-cli/internal/envvalues"
	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/spf13/cobra"
)

func newAppsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "manage the project's apps catalog and per-env enabled state",
		Long: `subcommands for inspecting and editing the apps catalog (apps-manifest.yaml
at project root) and the per-env enabled state (env's application.yml).

apps-manifest.yaml is producer-curated — the upstream catalog of every app
sabokit can deploy. It is NOT where you set per-env enabled state. That
lives in environments/<env>/application.yml and gets propagated to
.enabled_apps.json when the application layer applies.`,
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
                 reads .enabled_apps.json from environments/<env>/.
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
		Short: "enable an app in the current env's application.yml",
		Long: `edits environments/<env>/application.yml:
  - if <name> is commented out (`+"`# <name>:`"+` block), uncomments the block
  - if disabled (enabled: false), flips to enabled: true
  - if absent, appends a minimal block (with a FIXME hostname)

prints a next-step hint. requires an env. validates <name> against the
upstream catalog (apps-manifest.yaml).`,
		Example: `  sabokit apps add vikunja
  sabokit --env staging apps add jitsi`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAppsAdd(args[0])
		},
	}
}

func newAppsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "disable an app in the current env's application.yml",
		Long: `sets enabled: false in <name>'s block in environments/<env>/
application.yml. inserts the line if the block is missing one. errors if
the app is already disabled or absent.

prints a next-step hint: 'sabokit up --layers application' applies the
removal (terraform destroys the app's resources, ansible skips it).`,
		Example: `  sabokit apps remove jitsi
  sabokit --env staging apps remove decidim`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAppsRemove(args[0])
		},
	}
}

func runAppsAdd(name string) error {
	p, err := project.Load()
	if err != nil {
		return err
	}
	envDir, err := requireEnvDir(p)
	if err != nil {
		return err
	}
	if err := requireInCatalog(p, name); err != nil {
		return err
	}
	path := filepath.Join(envDir, "application.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	baseDomain := ""
	if values, err := envvalues.ForEnv(p.Root, p.EnvName(globals.Env)); err == nil {
		baseDomain = values.String("base_domain")
	}
	out, err := appsyaml.AddApp(string(content), name, baseDomain)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return err
	}
	fmt.Printf("enabled %s in %s\n", name, path)
	fmt.Printf("next: check the block's hostname, then 'sabokit up --layers application'\n")
	return nil
}

func runAppsRemove(name string) error {
	p, err := project.Load()
	if err != nil {
		return err
	}
	envDir, err := requireEnvDir(p)
	if err != nil {
		return err
	}
	path := filepath.Join(envDir, "application.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	out, err := appsyaml.RemoveApp(string(content), name)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return err
	}
	fmt.Printf("disabled %s in %s\n", name, path)
	fmt.Printf("next: sabokit down --apps %s              # stop the containers\n", name)
	fmt.Printf("      sabokit up --layers application     # apply the removal in terraform\n")
	return nil
}

func requireEnvDir(p *project.Project) (string, error) {
	if p.EnvName(globals.Env) == "" {
		return "", fmt.Errorf("an env is required (pass --env or set default_env in .sabokit/config.yml)")
	}
	return p.WorkspaceDir(globals.Env)
}

func requireInCatalog(p *project.Project, name string) error {
	cat, err := p.Catalog()
	if err != nil {
		return err
	}
	for _, a := range cat {
		if a.ID == name {
			return nil
		}
	}
	return fmt.Errorf("%q is not in the apps catalog — run 'sabokit apps list' to see valid names", name)
}
