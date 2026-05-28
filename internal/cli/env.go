package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sheyaln/sabokit-cli/internal/docker"
	"github.com/sheyaln/sabokit-cli/internal/envvalues"
	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/sheyaln/sabokit-cli/internal/scw"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "manage environments under environments/<env>/",
	}
	cmd.AddCommand(newEnvAddCmd())
	return cmd
}

type envAddFlags struct {
	from       string
	skipBucket bool
}

func newEnvAddCmd() *cobra.Command {
	f := &envAddFlags{}
	cmd := &cobra.Command{
		Use:   "add <name> --from <existing-env>",
		Short: "scaffold a new env by carbon-copying an existing one",
		Long: `add <name> creates environments/<name>/ by carbon-copying the env root
from environments/<existing-env>/ (config.tf, env.tf, *.tf — all byte-identical
across envs) and skipping per-env state, locks, and runtime artifacts
(backend.hcl, inventory.ini, .terraform/, *.tfstate*, *.tfvars).

backend.hcl is regenerated to point at "<org-slug>-tfstate-<name>", and a
"<name>:" block is appended to environments/env-values.yml with placeholder
project_id/domains for you to fill in (each env needs a distinct project). No
env-specific value lives in the env dir, so the copy carries nothing to rewrite.

a fresh Scaleway state bucket is created for the new env (--skip-bucket
to opt out).`,
		Example: `  sabokit env add staging --from prod
  sabokit env add ephemeral --from prod --skip-bucket`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnvAdd(args[0], f)
		},
	}
	cmd.Flags().StringVar(&f.from, "from", "", "existing env to carbon-copy from (required)")
	cmd.Flags().BoolVar(&f.skipBucket, "skip-bucket", false, "skip TF-state bucket creation (caller will provision manually)")
	_ = cmd.MarkFlagRequired("from")
	return cmd
}

func runEnvAdd(name string, f *envAddFlags) error {
	if err := validateEnvName(name); err != nil {
		return err
	}
	if f.from == "" {
		return fmt.Errorf("--from <existing-env> is required")
	}
	if name == f.from {
		return fmt.Errorf("--from %q must differ from new env name", f.from)
	}

	proj, err := project.Load()
	if err != nil {
		return err
	}

	srcDir := filepath.Join(proj.Root, "environments", f.from)
	dstDir := filepath.Join(proj.Root, "environments", name)

	if fi, err := os.Stat(srcDir); err != nil || !fi.IsDir() {
		return fmt.Errorf("source env not found: environments/%s", f.from)
	}
	if _, err := os.Stat(dstDir); err == nil {
		return fmt.Errorf("environments/%s already exists", name)
	}

	orgSlug, err := readOrgSlug(proj.Root)
	if err != nil {
		return err
	}
	bucket := fmt.Sprintf("%s-tfstate-%s", orgSlug, name)
	if len(bucket) > 63 {
		return fmt.Errorf("derived bucket name %q is %d chars; scaleway caps at 63 — shorten env name or org-slug", bucket, len(bucket))
	}

	done := step(fmt.Sprintf("carbon-copying environments/%s/ → environments/%s/", f.from, name))
	copied, err := copyEnvTree(srcDir, dstDir)
	if err != nil {
		return fmt.Errorf("copy env tree: %w", err)
	}
	done()

	done = step(fmt.Sprintf("appending %q block to environments/env-values.yml", name))
	if err := addEnvValuesBlock(proj.Root, name, f.from); err != nil {
		return err
	}
	done()

	done = step(fmt.Sprintf("regenerating backend.hcl (bucket %s)", bucket))
	if err := writeBackendHCL(dstDir, bucket); err != nil {
		return err
	}
	done()

	if !f.skipBucket {
		done = step(fmt.Sprintf("ensuring TF-state bucket %s", bucket))
		if err := ensureEnvStateBucket(bucket, proj.Config.Scaleway.Region); err != nil {
			return fmt.Errorf("create state bucket: %w", err)
		}
		done()
	}

	fmt.Printf("\ndone. copied %d file(s) into environments/%s/.\n", copied, name)
	fmt.Printf("    edit the %q block in environments/env-values.yml — set a real\n", name)
	fmt.Println("    scaleway_project_id, base_domain, gateway_domain (placeholders were written).")
	fmt.Printf("    secrets come from the environment / .envrc before 'sabokit --env %s up'.\n", name)
	return nil
}

// envNameRe restricts new env names to lowercase alphanumeric + hyphens,
// ≤30 chars. Matches the bucket-name discipline scw enforces upstream.
var envNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,29}$`)

func validateEnvName(name string) error {
	if name == "" {
		return fmt.Errorf("env name is required")
	}
	if !envNameRe.MatchString(name) {
		return fmt.Errorf("invalid env name %q: must be lowercase alphanumeric + hyphens, ≤30 chars, starting with alphanumeric", name)
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("env name must not contain path separators: %q", name)
	}
	return nil
}

// envCopySkipExact lists basenames that must never travel across envs.
var envCopySkipExact = map[string]struct{}{
	"backend.hcl":         {},
	"inventory.ini":       {},
	".terraform.lock.hcl": {},
	".envrc":              {},
	".env":                {},
	".ansible-vars.json":  {},
	".tf-output.json":     {},
}

// envCopySkipDir lists directory basenames pruned during the walk.
var envCopySkipDir = map[string]struct{}{
	".terraform": {},
	".git":       {},
}

// shouldSkipEnvFile decides whether a file under the source env dir is
// excluded from the carbon copy. Decisions follow the FEATURES.md
// scaffolding non-negotiables: tfvars are secrets (per-env), state never
// travels, locks/runtime artifacts regenerate downstream.
func shouldSkipEnvFile(rel string) bool {
	base := filepath.Base(rel)
	if _, ok := envCopySkipExact[base]; ok {
		return true
	}
	if strings.HasSuffix(base, ".tfvars") {
		return true
	}
	if strings.HasPrefix(base, "terraform.tfstate") {
		return true
	}
	if strings.HasSuffix(base, ".tfstate") || strings.HasSuffix(base, ".tfstate.backup") {
		return true
	}
	if strings.HasPrefix(base, ".terraform") {
		return true
	}
	return false
}

// copyEnvTree walks src and copies every non-skipped file into dst,
// preserving file modes and directory structure. Returns the count of
// files copied.
func copyEnvTree(src, dst string) (int, error) {
	count := 0
	err := filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		base := filepath.Base(rel)
		if info.IsDir() {
			if _, skip := envCopySkipDir[base]; skip {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), info.Mode())
		}
		if shouldSkipEnvFile(rel) {
			return nil
		}
		if err := copyEnvFile(path, filepath.Join(dst, rel), info.Mode()); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}

func copyEnvFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// addEnvValuesBlock appends a "<name>:" block to environments/env-values.yml,
// seeded from the source env's slice but with placeholder project_id/domains —
// each env needs a distinct project, and the operator fills the real values.
// Appends text (rather than re-marshalling the whole file) so existing
// comments + env blocks survive untouched.
func addEnvValuesBlock(projectRoot, name, from string) error {
	path := envvalues.Path(projectRoot)
	existing, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	all, err := envvalues.Load(projectRoot)
	if err != nil {
		return err
	}
	if _, exists := all[name]; exists {
		return fmt.Errorf("env %q already has a block in %s", name, path)
	}
	src := all[from]
	block := map[string]any{
		"scaleway_project_id": "REPLACE-with-" + name + "-project-uuid",
		"base_domain":         "CHANGEME-" + name + "-base-domain",
		"gateway_domain":      "CHANGEME-" + name + "-gateway-domain",
		"infra_email":         sliceStr(src, "infra_email", "ops@example.org"),
		"scaleway_region":     sliceStr(src, "scaleway_region", "fr-par"),
		"scaleway_zone":       sliceStr(src, "scaleway_zone", "fr-par-1"),
	}
	body, err := yaml.Marshal(map[string]map[string]any{name: block})
	if err != nil {
		return err
	}
	out := existing
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	out = append(out, '\n')
	out = append(out, body...)
	return os.WriteFile(path, out, 0o644)
}

func sliceStr(s envvalues.Slice, key, def string) string {
	if s != nil {
		if v := s.String(key); v != "" {
			return v
		}
	}
	return def
}

// writeBackendHCL regenerates backend.hcl for the new env, matching the
// shape `sabokit init` writes (kept in sync deliberately — both call
// sites materialise the same template).
func writeBackendHCL(envDir, bucket string) error {
	content := fmt.Sprintf(`# Generated by sabokit env add. Bucket is created in the same step; key is
# the canonical layout. Override the bucket name only if a naming collision
# forces you to (S3 namespace is global).

bucket = %q
key    = "stack/terraform.tfstate"
`, bucket)
	return os.WriteFile(filepath.Join(envDir, "backend.hcl"), []byte(content), 0o644)
}

func ensureEnvStateBucket(bucket, region string) error {
	if os.Getenv("SCW_ACCESS_KEY") == "" || os.Getenv("SCW_SECRET_KEY") == "" {
		return fmt.Errorf("SCW_ACCESS_KEY and SCW_SECRET_KEY must be set in the environment (or pass --skip-bucket to provision the bucket yourself)")
	}
	if err := docker.Preflight(); err != nil {
		return err
	}
	client := scw.New(globals.ScwImage, globals.Platform)
	return client.CreateBucket(bucket, region)
}

// readOrgSlug pulls org_slug from any committed config.tf under
// environments/<env>/ — every env's config.tf carries the same org_slug
// (it's the cross-env identifier). Falls back to scanning each env until
// it finds a populated value.
func readOrgSlug(projectRoot string) (string, error) {
	envs, err := os.ReadDir(filepath.Join(projectRoot, "environments"))
	if err != nil {
		return "", fmt.Errorf("read environments/: %w", err)
	}
	re := regexp.MustCompile(`(?m)^\s*org_slug\s*=\s*"([^"]+)"`)
	for _, e := range envs {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(projectRoot, "environments", e.Name(), "config.tf")
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if m := re.FindStringSubmatch(string(raw)); m != nil && m[1] != "" {
			return m[1], nil
		}
	}
	return "", fmt.Errorf("could not derive org_slug from any environments/*/config.tf — populate one env first or pass --skip-bucket")
}
