package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "manage .sabokit/config.yml",
		Long: `subcommands for the per-project sabokit config file. config.yml carries
the sabokit-cli-side metadata (project name, base domain, default env,
scaleway region/zone, ssh user/key); per-env terraform config lives in
environments/<env>/config.tf, not here.`,
	}
	cmd.AddCommand(newConfigInitCmd(), newConfigShowCmd())
	return cmd
}

type configInitFlags struct {
	project        string
	baseDomain     string
	defaultEnv     string
	region         string
	zone           string
	sshUser        string
	sshKey         string
	nonInteractive bool
	force          bool
}

func newConfigInitCmd() *cobra.Command {
	f := &configInitFlags{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "interactively generate .sabokit/config.yml in cwd",
		Long: `prompts for project metadata and writes .sabokit/config.yml in the
current directory. fields you pass via flags are not re-prompted.

use this on an existing consumer-template checkout that doesn't yet have a
sabokit-cli config — e.g. when you ran 'git clone' of your infra repo
instead of 'sabokit init'.

refuses to overwrite an existing config without --force.`,
		Example: `  sabokit config init
  sabokit config init --project my-stack --base-domain example.com --env prod
  sabokit config init --non-interactive --project my-stack --base-domain example.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigInit(f)
		},
	}
	cmd.Flags().StringVar(&f.project, "project", "", "project name (default: cwd basename)")
	cmd.Flags().StringVar(&f.baseDomain, "base-domain", "", "base domain for the stack")
	cmd.Flags().StringVar(&f.defaultEnv, "default-env", "", "default env (eg. prod); blank for unset")
	cmd.Flags().StringVar(&f.region, "region", "fr-par", "scaleway region")
	cmd.Flags().StringVar(&f.zone, "zone", "", "scaleway zone (default: <region>-1)")
	cmd.Flags().StringVar(&f.sshUser, "ssh-user", "ubuntu", "ssh user")
	cmd.Flags().StringVar(&f.sshKey, "ssh-key", "~/.ssh/id_ed25519", "ssh key path")
	cmd.Flags().BoolVar(&f.nonInteractive, "non-interactive", false, "skip prompts; require all values via flags")
	cmd.Flags().BoolVar(&f.force, "force", false, "overwrite an existing .sabokit/config.yml")
	return cmd
}

func runConfigInit(f *configInitFlags) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	path := filepath.Join(cwd, ".sabokit", "config.yml")
	if _, err := os.Stat(path); err == nil && !f.force {
		return fmt.Errorf("%s already exists — pass --force to overwrite", path)
	}

	inputs := configInputs{
		project:    f.project,
		baseDomain: f.baseDomain,
		defaultEnv: f.defaultEnv,
		region:     f.region,
		zone:       f.zone,
		sshUser:    f.sshUser,
		sshKey:     f.sshKey,
	}
	if err := promptConfigInputs(&inputs, !f.nonInteractive); err != nil {
		return err
	}

	written, err := writeConfigYAML(cwd, inputs)
	if err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", written)
	return nil
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "print the loaded .sabokit/config.yml + the path it came from",
		Long: `walks up from cwd to find .sabokit/config.yml, then prints its contents
verbatim with a header showing the path. errors if no config is found.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigShow()
		},
	}
}

func runConfigShow() error {
	p, err := project.Load()
	if err != nil {
		return err
	}
	path := filepath.Join(p.Root, ".sabokit", "config.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fmt.Printf("# %s\n", path)
	fmt.Print(string(raw))
	return nil
}
