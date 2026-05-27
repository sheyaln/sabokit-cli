package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/sheyaln/sabokit-cli/internal/template"
	"github.com/spf13/cobra"
)

type upFlags struct {
	skipPreflight bool
	skipUp        bool
	skipConfigure bool
	templateTag   string
}

func newUpCmd() *cobra.Command {
	f := &upFlags{}
	cmd := &cobra.Command{
		Use:   "up",
		Short: "run preflight.sh, up.sh, configure.sh for the current env",
		Long: `chains the three consumer-template scripts in the current env dir:
  preflight.sh   one-time deps + SSH key + DNS placeholder check
  up.sh          terraform apply (base + identity_bootstrap) + bootstrap ansible
  configure.sh   authentik flows/brand/groups + per-app TF apply

these scripts execute LOCALLY, not inside the runner image. they expect:
  - terraform, ansible, scw, jq, python3, awk, nc, ssh, ssh-keygen on PATH
  - SCW_ACCESS_KEY / SCW_SECRET_KEY in the environment
  - FED_COMMONS_DIR pointing at a sabokit repo checkout (sabokit auto-clones
    one at .sabokit/sabokit-repo/ on first run; pin the tag via --template-tag,
    default ` + template.DefaultTag + `)

step-skip flags are for re-runs after a partial failure. each script is
idempotent — re-running the chain in full is also safe.`,
		Example: `  sabokit up                     # full chain
  sabokit up --skip-preflight    # resume after deps verified
  sabokit up --skip-up           # just re-run configure
  sabokit --env staging up`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUp(f)
		},
	}
	cmd.Flags().BoolVar(&f.skipPreflight, "skip-preflight", false, "skip preflight.sh")
	cmd.Flags().BoolVar(&f.skipUp, "skip-up", false, "skip up.sh")
	cmd.Flags().BoolVar(&f.skipConfigure, "skip-configure", false, "skip configure.sh")
	cmd.Flags().StringVar(&f.templateTag, "template-tag", template.DefaultTag, "git tag to clone sabokit at when seeding FED_COMMONS_DIR")
	return cmd
}

func runUp(f *upFlags) error {
	p, err := project.Load()
	if err != nil {
		return err
	}
	if p.EnvName(globals.Env) == "" {
		return fmt.Errorf("up requires an env (pass --env or set default_env in .sabokit/config.yml)")
	}
	envDir, err := p.WorkspaceDir(globals.Env)
	if err != nil {
		return err
	}

	fedDir, err := ensureFedCommonsCheckout(p, f.templateTag)
	if err != nil {
		return err
	}

	steps := []struct {
		name string
		skip bool
	}{
		{"preflight.sh", f.skipPreflight},
		{"up.sh", f.skipUp},
		{"configure.sh", f.skipConfigure},
	}
	for _, s := range steps {
		if s.skip {
			fmt.Printf("==> skipping %s\n", s.name)
			continue
		}
		fmt.Printf("==> %s\n", s.name)
		if err := runEnvScript(envDir, fedDir, s.name); err != nil {
			return fmt.Errorf("%s failed: %w", s.name, err)
		}
	}
	return nil
}

// ensureFedCommonsCheckout returns the path to a local sabokit/ checkout
// suitable for use as FED_COMMONS_DIR. Caches at .sabokit/sabokit-repo/ in
// the project root. Honors a pre-existing FED_COMMONS_DIR env var.
func ensureFedCommonsCheckout(p *project.Project, tag string) (string, error) {
	if existing := os.Getenv("FED_COMMONS_DIR"); existing != "" {
		if _, err := os.Stat(existing); err == nil {
			return existing, nil
		}
	}
	dir := filepath.Join(p.Root, ".sabokit", "sabokit-repo")
	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		return dir, nil
	}
	fmt.Printf("==> cloning sabokit @ %s for FED_COMMONS_DIR\n", tag)
	return template.Clone("", tag, dir)
}

func runEnvScript(envDir, fedDir, scriptName string) error {
	path := filepath.Join(envDir, scriptName)
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("script not found: %s", path)
	}
	if err := ensureExecutable(path); err != nil {
		return err
	}
	c := exec.Command("bash", "./"+scriptName)
	c.Dir = envDir
	c.Env = append(os.Environ(), "FED_COMMONS_DIR="+fedDir)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func ensureExecutable(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	mode := fi.Mode()
	if mode&0o111 != 0 {
		return nil
	}
	return os.Chmod(path, mode|0o755)
}
