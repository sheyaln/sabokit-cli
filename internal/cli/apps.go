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
	}
	cmd.AddCommand(newAppsListCmd(), newAppsAddCmd(), newAppsRemoveCmd())
	return cmd
}

func newAppsListCmd() *cobra.Command {
	var enabledOnly bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list apps in the project manifest (available + enabled)",
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
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("apps add")
		},
	}
}

func newAppsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "remove an app from apps-manifest.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("apps remove")
		},
	}
}
