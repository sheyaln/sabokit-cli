package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sheyaln/sabokit-cli/internal/template"
	"github.com/spf13/cobra"
)

type initFlags struct {
	templateRepo   string
	templateTag    string
	baseDomain     string
	region         string
	zone           string
	sshUser        string
	sshKey         string
	env            string
	nonInteractive bool
}

func newInitCmd() *cobra.Command {
	f := &initFlags{}
	cmd := &cobra.Command{
		Use:   "init <project-name>",
		Short: "scaffold a new consumer project",
		Long: `clones consumer-template/ from the upstream sabokit repo at a pinned tag
and copies it into <project-name>/. then prompts for project metadata
(or accepts flags) and writes .sabokit/config.yml at the project root.

defaults:
  --from-template-repo  https://github.com/sheyaln/sabokit
  --from-template-tag   ` + template.DefaultTag + `
  --region              fr-par
  --ssh-user            root
  --ssh-key             ~/.ssh/id_ed25519
  --env                 (empty — no env scaffolded by default)

with --env <name>: copies environments/_template/ to environments/<name>/
and writes default_env: <name> in .sabokit/config.yml so subsequent
sabokit commands target that env automatically.

set --non-interactive to skip prompts (all other required values must be
passed as flags). consumer-template is fetched fresh via 'git clone
--depth 1 --branch <tag>'; the .git directory is stripped from the result.

next steps after init are printed at the end and follow consumer-template's
own README (cp config.tf.example config.tf, edit, run preflight/up/configure).`,
		Example: `  sabokit init my-stack
  sabokit init my-stack --env prod --base-domain example.com
  sabokit init my-stack --non-interactive --base-domain example.com --env prod`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(args[0], f)
		},
	}
	cmd.Flags().StringVar(&f.templateRepo, "from-template-repo", template.DefaultRepo, "git repo to clone consumer-template from")
	cmd.Flags().StringVar(&f.templateTag, "from-template-tag", template.DefaultTag, "tag/branch to clone")
	cmd.Flags().StringVar(&f.baseDomain, "base-domain", "", "base domain for the stack (eg. example.com)")
	cmd.Flags().StringVar(&f.region, "region", "fr-par", "scaleway region")
	cmd.Flags().StringVar(&f.zone, "zone", "", "scaleway zone (default: <region>-1)")
	cmd.Flags().StringVar(&f.sshUser, "ssh-user", "root", "ssh user for ansible / sabokit ssh")
	cmd.Flags().StringVar(&f.sshKey, "ssh-key", "~/.ssh/id_ed25519", "ssh key path (sabokit will mount this into the runner)")
	cmd.Flags().StringVar(&f.env, "env", "", "also bootstrap environments/<name>/ from _template and set default_env")
	cmd.Flags().BoolVar(&f.nonInteractive, "non-interactive", false, "skip prompts; require all values via flags")
	return cmd
}

func runInit(projectName string, f *initFlags) error {
	if err := validateProjectName(projectName); err != nil {
		return err
	}
	target, err := filepath.Abs(projectName)
	if err != nil {
		return err
	}
	if err := ensureTargetReady(target); err != nil {
		return err
	}

	inputs := configInputs{
		project:    projectName,
		baseDomain: f.baseDomain,
		defaultEnv: f.env,
		region:     coalesce(f.region, "fr-par"),
		zone:       f.zone,
		sshUser:    coalesce(f.sshUser, "root"),
		sshKey:     coalesce(f.sshKey, "~/.ssh/id_ed25519"),
	}
	if err := promptConfigInputs(&inputs, !f.nonInteractive); err != nil {
		return err
	}
	// init sources project name from positional arg, not prompts — pin it.
	inputs.project = projectName
	if f.env != "" {
		inputs.defaultEnv = f.env
	}

	fmt.Printf("cloning %s @ %s\n", f.templateRepo, f.templateTag)
	src, err := template.Fetch(template.FetchOptions{Repo: f.templateRepo, Tag: f.templateTag})
	if err != nil {
		return err
	}
	defer template.CleanupParent(src)

	fmt.Printf("copying consumer-template/ → %s\n", target)
	if err := template.CopyTree(src, target); err != nil {
		return err
	}

	if f.env != "" {
		if err := bootstrapEnv(target, f.env); err != nil {
			return err
		}
		fmt.Printf("bootstrapped environments/%s/ from _template\n", f.env)
	}

	if _, err := writeConfigYAML(target, inputs); err != nil {
		return err
	}

	fmt.Printf("\ndone. next steps:\n")
	fmt.Printf("  cd %s\n", projectName)
	if f.env != "" {
		fmt.Printf("  cd environments/%s\n", f.env)
	} else {
		fmt.Printf("  cp -r environments/_template environments/<env>\n")
		fmt.Printf("  cd environments/<env>\n")
	}
	fmt.Printf("  cp config.tf.example     config.tf      && $EDITOR config.tf\n")
	fmt.Printf("  cp backend.hcl.example   backend.hcl    && $EDITOR backend.hcl\n")
	fmt.Printf("  cp inventory.ini.example inventory.ini\n")
	fmt.Printf("  chmod +x preflight.sh up.sh configure.sh\n")
	fmt.Printf("  ./preflight.sh && ./up.sh && ./configure.sh\n")
	fmt.Printf("\nsee %s/README.md and environments/_template/README.md for detail.\n", projectName)
	return nil
}

func bootstrapEnv(projectRoot, envName string) error {
	if strings.ContainsAny(envName, "/\\") {
		return fmt.Errorf("env name must not contain path separators: %q", envName)
	}
	src := filepath.Join(projectRoot, "environments", "_template")
	dst := filepath.Join(projectRoot, "environments", envName)
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("environments/%s already exists", envName)
	}
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("environments/_template not found in template (tag %s) — cannot bootstrap env", template.DefaultTag)
	}
	return template.CopyTree(src, dst)
}

func validateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name is required")
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("project name must not contain path separators: %q", name)
	}
	return nil
}

func ensureTargetReady(target string) error {
	fi, err := os.Stat(target)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("target %s exists and is not a directory", target)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("target %s exists and is not empty", target)
	}
	return nil
}

func coalesce(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
