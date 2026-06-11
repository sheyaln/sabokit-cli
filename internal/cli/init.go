package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sheyaln/sabokit-cli/internal/docker"
	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/sheyaln/sabokit-cli/internal/scw"
	"github.com/sheyaln/sabokit-cli/internal/template"
	"github.com/sheyaln/sabokit-cli/internal/version"
	"github.com/spf13/cobra"
)

type initFlags struct {
	templateRepo   string
	templateTag    string
	baseDomain     string
	gatewayDomain  string
	scwProjectID   string
	orgSlug        string
	orgName        string
	infraEmail     string
	region         string
	zone           string
	sshUser        string
	sshKey         string
	envs           []string
	staging        bool
	skipBucket     bool
	nonInteractive bool
}

func newInitCmd() *cobra.Command {
	f := &initFlags{}
	cmd := &cobra.Command{
		Use:   "init <project-name>",
		Short: "scaffold a new sabokit project end-to-end (consumer-template + envs + state buckets)",
		Long: `clones consumer-template from the upstream sabokit repo, copies it into
<project-name>/, then scaffolds one or more environments end-to-end:

  environments/common.yml   org_slug + org_name substituted from prompts
  for each env (copied from environments/_template):
    env.yml                 per-env identity (project UUID, domains, email)
                            from prompts; later envs get placeholders
    <layer>/backend.hcl     generated ×4; bucket = "<org-slug>-tfstate-<env>"
    inventory.ini           not scaffolded — regenerated from terraform state
  creates the scaleway object bucket each env's TF state lives in
    (skip with --skip-bucket; requires SCW_ACCESS_KEY/SCW_SECRET_KEY).

after init, review environments/<env>/{env.yml,hosts.yml,infra.yml,
identity.yml,operations.yml,application.yml} — env.yml is identity/sizing,
the rest is per-layer shape (apps, tiers, watchers). then 'sabokit up'.

interactive mode (default) prompts for every required field; pass
--non-interactive plus the flags to script the same flow.`,
		Example: `  sabokit init my-stack
  sabokit init my-stack --env prod --staging
  sabokit init my-stack --non-interactive \
      --base-domain example.org --scaleway-project-id <uuid> \
      --org-slug acme --org-name "Acme Co" --infra-email ops@example.org \
      --env prod`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(args[0], f)
		},
	}
	cmd.Flags().StringVar(&f.templateRepo, "from-template-repo", template.DefaultRepo, "git repo to clone consumer-template from")
	cmd.Flags().StringVar(&f.templateTag, "from-template-tag", "", "blueprint tag/branch to clone (default: latest supported sabokit release)")
	cmd.Flags().StringVar(&f.baseDomain, "base-domain", "", "base domain (eg. example.org)")
	cmd.Flags().StringVar(&f.gatewayDomain, "gateway-domain", "", "Authentik gateway domain (default: auth.<base-domain>)")
	cmd.Flags().StringVar(&f.scwProjectID, "scaleway-project-id", "", "scaleway project UUID")
	cmd.Flags().StringVar(&f.orgSlug, "org-slug", "", "short org identifier used in resource names")
	cmd.Flags().StringVar(&f.orgName, "org-name", "", "human-readable org name shown in Authentik UI")
	cmd.Flags().StringVar(&f.infraEmail, "infra-email", "", "ops email used for LE registration + alerts")
	cmd.Flags().StringVar(&f.region, "region", "fr-par", "scaleway region")
	cmd.Flags().StringVar(&f.zone, "zone", "", "scaleway zone (default: <region>-1)")
	cmd.Flags().StringVar(&f.sshUser, "ssh-user", "ubuntu", "ssh user for ansible / sabokit ssh")
	cmd.Flags().StringVar(&f.sshKey, "ssh-key", "~/.ssh/id_ed25519", "ssh key path (sabokit will mount this into the runner)")
	cmd.Flags().StringSliceVar(&f.envs, "env", nil, "envs to scaffold (eg. --env prod or --env prod,staging); default: prod (or interactive)")
	cmd.Flags().BoolVar(&f.staging, "staging", false, "also scaffold a 'staging' env in addition to 'prod' (non-interactive shorthand)")
	cmd.Flags().BoolVar(&f.skipBucket, "skip-bucket", false, "skip TF-state bucket creation (caller will provision manually)")
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

	if err := promptInitInputs(projectName, f); err != nil {
		return err
	}
	envs := resolveEnvs(f)
	if len(envs) == 0 {
		return fmt.Errorf("at least one env is required")
	}

	if err := validateBucketNames(envs, f); err != nil {
		return err
	}

	if f.templateTag == "" {
		latest, err := template.LatestRef(f.templateRepo, version.SupportedBlueprintMax)
		if err != nil {
			return fmt.Errorf("resolve latest supported sabokit release: %w", err)
		}
		if latest == "" {
			fmt.Fprintf(os.Stderr, "    no v%s.* sabokit release found yet — scaffolding from master\n", version.SupportedBlueprintMax)
			latest = "master"
		}
		f.templateTag = latest
	}

	done := step(fmt.Sprintf("cloning consumer-template @ %s", f.templateTag))
	src, err := template.Fetch(template.FetchOptions{Repo: f.templateRepo, Tag: f.templateTag})
	if err != nil {
		return err
	}
	defer template.CleanupParent(src)
	done()

	done = step(fmt.Sprintf("copying into %s", relPath(target)))
	if err := template.CopyTree(src, target); err != nil {
		return err
	}
	done()

	done = step("writing environments/common.yml (org identity)")
	if err := materialiseCommonYML(target, f); err != nil {
		return err
	}
	done()

	for i, env := range envs {
		done = step(fmt.Sprintf("scaffolding environments/%s/ (env.yml, per-layer backend.hcl)", env))
		if err := scaffoldEnv(target, env, f, i == 0); err != nil {
			return fmt.Errorf("scaffold env %s: %w", env, err)
		}
		done()
	}

	configInputs := configInputs{
		project:    projectName,
		baseDomain: f.baseDomain,
		defaultEnv: envs[0],
		region:     f.region,
		zone:       f.zone,
		sshUser:    f.sshUser,
		sshKey:     f.sshKey,
	}
	done = step("writing .sabokit/config.yml")
	if _, err := writeConfigYAML(target, configInputs); err != nil {
		return err
	}
	done()

	done = step("writing .gitignore, .envrc.example, README.md")
	if err := writeProjectScaffolds(target, projectName, envs, f); err != nil {
		return err
	}
	done()

	if !f.skipBucket {
		if err := ensureStateBuckets(envs, f); err != nil {
			return fmt.Errorf("create state buckets: %w", err)
		}
	}

	fmt.Printf("\ndone. next: cd %s && sabokit up\n", projectName)
	fmt.Println("    (per-env identity/sizing is environments/<env>/env.yml; the layer")
	fmt.Println("     shape — apps, tiers, watchers — is the sibling per-layer YAML files)")
	return nil
}

// step prints "==> <label>" and returns a closure that prints "    ok" on
// success. Callers wrap each substantive step so output is uniform.
func step(label string) func() {
	fmt.Printf("==> %s\n", label)
	return func() {
		fmt.Println("    ok")
	}
}

func relPath(abs string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return abs
	}
	rel, err := filepath.Rel(cwd, abs)
	if err != nil {
		return abs
	}
	return rel
}

// validateBucketNames checks the derived state-bucket name fits scw's
// 63-char limit BEFORE the long-running clone runs. Fails fast.
func validateBucketNames(envs []string, f *initFlags) error {
	if f.skipBucket {
		return nil
	}
	for _, env := range envs {
		name := fmt.Sprintf("%s-tfstate-%s", f.orgSlug, env)
		if len(name) > 63 {
			return fmt.Errorf("derived bucket name %q is %d chars; scaleway caps at 63 — shorten --org-slug (currently %q)", name, len(name), f.orgSlug)
		}
	}
	return nil
}

// writeProjectScaffolds drops the standard top-level files every sabokit
// project needs: .gitignore, .envrc.example, README.md.
func writeProjectScaffolds(target, projectName string, envs []string, f *initFlags) error {
	if err := writeGitignore(target); err != nil {
		return err
	}
	if err := writeEnvrcExample(target); err != nil {
		return err
	}
	return writeOperatorREADME(target, projectName, envs, f)
}

func writeGitignore(target string) error {
	path := filepath.Join(target, ".gitignore")
	content := `# terraform state lives in scaleway object storage — never commit local copies
terraform.tfstate
terraform.tfstate.*
*.tfstate.backup
.terraform/
.terraform.lock.hcl

# sabokit-derived runtime files (regenerated from terraform state every up/deploy)
inventory.ini
.enabled_apps.json

# per-env backend config is generated (sabokit init / env add / up preflight)
backend.hcl

# terraform plans (often contain secrets in the diff)
*.tfplan

# per-env values live in the committed environments/<env>/env.yml; tfvars
# are not used by sabokit. treat any stray *.tfvars as secret — never commit.
*.tfvars
*.tfvars.json

# secrets come from the environment / .envrc (direnv) — never commit.
.envrc
!.envrc.example

# editors + OS
.idea/
.vscode/
.DS_Store
`
	return os.WriteFile(path, []byte(content), 0o644)
}

func writeEnvrcExample(target string) error {
	path := filepath.Join(target, ".envrc.example")
	content := `# copy to .envrc, fill in, then 'direnv allow' (or 'source .envrc').
# never commit .envrc — it contains long-lived scaleway credentials.

export SCW_ACCESS_KEY="SCW..."
export SCW_SECRET_KEY="..."
export SCW_DEFAULT_PROJECT_ID="..."
export SCW_DEFAULT_REGION="fr-par"
export SCW_DEFAULT_ZONE="fr-par-1"
`
	return os.WriteFile(path, []byte(content), 0o644)
}

func writeOperatorREADME(target, projectName string, envs []string, f *initFlags) error {
	path := filepath.Join(target, "README.md")
	content := fmt.Sprintf(`# %s

sabokit-managed self-hosting stack.

## envs

%s

%s is the default (sabokit-cli's `+"`default_env`"+`).

## operating

sabokit-cli is the only tool you need installed. See
https://github.com/sheyaln/sabokit-cli for install + command reference.

`+"```"+`bash
sabokit up                          # provision + configure the default env
sabokit status                      # tf outputs + container state
sabokit deploy --apps espocrm       # redeploy one app
sabokit logs <app> -f               # follow logs
sabokit secrets list                # what's in scaleway secret manager
sabokit destroy --apps <name>       # tear down one app
sabokit --env staging up            # operate against a different env
`+"```"+`

## per-env config

each `+"`environments/<env>/`"+` directory carries committed YAML config plus
four terraform layer roots (`+"`infra`"+` → `+"`identity`"+` → `+"`operations`"+` → `+"`application`"+`),
each with its own remote state. secrets come from the environment /
`+"`.envrc`"+` — never from these files.

- `+"`env.yml`"+`           — per-env identity + sizing (project UUID, domains, instance types)
- `+"`hosts.yml`"+`         — compute topology (identical across envs)
- `+"`infra.yml`"+` / `+"`identity.yml`"+` / `+"`operations.yml`"+` / `+"`application.yml`"+` — per-layer shape (apps, tiers, watchers)
- `+"`<layer>/backend.hcl`"+` — generated (gitignored); remote-state bucket %s-tfstate-<env>, one key per layer
- `+"`inventory.ini`"+` / `+"`.enabled_apps.json`"+` — generated from terraform state; do not edit
`, projectName, formatBulletList(envs), envs[0], f.orgSlug)
	return os.WriteFile(path, []byte(content), 0o644)
}

func formatBulletList(items []string) string {
	var b strings.Builder
	for _, it := range items {
		fmt.Fprintf(&b, "- `%s`\n", it)
	}
	return b.String()
}

func promptInitInputs(projectName string, f *initFlags) error {
	if f.zone == "" {
		f.zone = f.region + "-1"
	}
	if f.orgSlug == "" {
		f.orgSlug = projectName
	}
	if f.orgName == "" {
		f.orgName = strings.Title(strings.ReplaceAll(projectName, "-", " "))
	}

	if f.nonInteractive {
		missing := []string{}
		if f.baseDomain == "" {
			missing = append(missing, "--base-domain")
		}
		if f.scwProjectID == "" {
			missing = append(missing, "--scaleway-project-id")
		}
		if f.infraEmail == "" {
			missing = append(missing, "--infra-email")
		}
		if len(missing) > 0 {
			return fmt.Errorf("missing required flags in --non-interactive mode: %s", strings.Join(missing, ", "))
		}
	} else {
		r := bufio.NewReader(os.Stdin)
		if f.baseDomain == "" {
			f.baseDomain = prompt(r, "base domain (eg. example.org): ", "")
			if f.baseDomain == "" {
				return fmt.Errorf("base domain is required")
			}
		}
		if f.gatewayDomain == "" {
			def := "auth." + f.baseDomain
			f.gatewayDomain = prompt(r, fmt.Sprintf("Authentik gateway domain [%s]: ", def), def)
		}
		if f.infraEmail == "" {
			f.infraEmail = prompt(r, "ops email for Let's Encrypt / alerts: ", "")
			if f.infraEmail == "" {
				return fmt.Errorf("infra email is required")
			}
		}
		if f.scwProjectID == "" {
			f.scwProjectID = prompt(r, "scaleway project UUID: ", "")
			if f.scwProjectID == "" {
				return fmt.Errorf("scaleway project UUID is required")
			}
		}
		f.orgSlug = prompt(r, fmt.Sprintf("org slug [%s]: ", f.orgSlug), f.orgSlug)
		f.orgName = prompt(r, fmt.Sprintf("org name [%s]: ", f.orgName), f.orgName)
		f.region = prompt(r, fmt.Sprintf("scaleway region [%s]: ", f.region), f.region)
		zoneDefault := f.region + "-1"
		f.zone = prompt(r, fmt.Sprintf("scaleway zone [%s]: ", zoneDefault), zoneDefault)
		f.sshUser = prompt(r, fmt.Sprintf("ssh user [%s]: ", f.sshUser), f.sshUser)
		f.sshKey = prompt(r, fmt.Sprintf("ssh key path [%s]: ", f.sshKey), f.sshKey)
	}

	if f.gatewayDomain == "" {
		f.gatewayDomain = "auth." + f.baseDomain
	}
	return nil
}

func resolveEnvs(f *initFlags) []string {
	envs := append([]string{}, f.envs...)
	if f.staging && !contains(envs, "staging") {
		if len(envs) == 0 {
			envs = []string{"prod"}
		}
		envs = append(envs, "staging")
	}
	if len(envs) > 0 {
		return envs
	}
	if f.nonInteractive {
		return []string{"prod"}
	}
	r := bufio.NewReader(os.Stdin)
	first := prompt(r, "first env name [prod]: ", "prod")
	envs = []string{first}
	if first == "prod" {
		ans := prompt(r, "also scaffold a staging env? [Y/n]: ", "y")
		if !strings.HasPrefix(strings.ToLower(ans), "n") {
			envs = append(envs, "staging")
		}
	}
	return envs
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// scaffoldEnv copies environments/_template into environments/<env> and
// materialises env.yml + the four per-layer backend.hcl files. The primary
// env gets the prompted values; later envs get a placeholder project_id
// (each env needs a distinct project).
func scaffoldEnv(projectRoot, envName string, f *initFlags, primary bool) error {
	if strings.ContainsAny(envName, "/\\") {
		return fmt.Errorf("env name must not contain path separators: %q", envName)
	}
	src := filepath.Join(projectRoot, "environments", "_template")
	dst := filepath.Join(projectRoot, "environments", envName)
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("environments/%s already exists", envName)
	}
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("environments/_template not found in template (tag %s)", f.templateTag)
	}
	if err := template.CopyTree(src, dst); err != nil {
		return err
	}
	if err := materialiseEnvYML(dst, envName, f, primary); err != nil {
		return err
	}
	bucket := fmt.Sprintf("%s-tfstate-%s", f.orgSlug, envName)
	for _, layer := range project.Layers {
		if err := writeLayerBackendHCL(dst, layer, bucket); err != nil {
			return err
		}
	}
	return nil
}

// materialiseEnvYML substitutes the prompted per-env identity into the
// copied env.yml. Textual per-key replacement keeps the template's comments
// (sizing guidance, subnet caveats) intact.
func materialiseEnvYML(envDir, envName string, f *initFlags, primary bool) error {
	path := filepath.Join(envDir, "env.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w (template may have shifted)", path, err)
	}
	projectID := f.scwProjectID
	if !primary {
		projectID = "REPLACE-with-" + envName + "-project-uuid"
	}
	content := string(raw)
	for k, v := range map[string]string{
		"scaleway_project_id": projectID,
		"base_domain":         f.baseDomain,
		"identity_domain":     f.gatewayDomain,
		"infra_email":         f.infraEmail,
		"scaleway_region":     f.region,
		"scaleway_zone":       f.zone,
	} {
		if v != "" {
			content = setYAMLScalar(content, k, v)
		}
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// materialiseCommonYML substitutes the org identity into the cross-env
// environments/common.yml.
func materialiseCommonYML(target string, f *initFlags) error {
	path := filepath.Join(target, "environments", "common.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w (template may have shifted)", path, err)
	}
	content := string(raw)
	for k, v := range map[string]string{
		"org_slug": f.orgSlug,
		"org_name": f.orgName,
	} {
		if v != "" {
			content = setYAMLScalar(content, k, v)
		}
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// ensureStateBuckets creates one Scaleway Object Storage bucket per env
// to back the env's terraform state. Idempotent — already-existing buckets
// are skipped silently.
func ensureStateBuckets(envs []string, f *initFlags) error {
	if os.Getenv("SCW_ACCESS_KEY") == "" || os.Getenv("SCW_SECRET_KEY") == "" {
		return fmt.Errorf("SCW_ACCESS_KEY and SCW_SECRET_KEY must be set in the environment (or pass --skip-bucket to provision buckets yourself)")
	}
	if err := docker.Preflight(); err != nil {
		return err
	}
	client := scw.New(globals.ScwImage, globals.Platform)
	for _, env := range envs {
		bucket := fmt.Sprintf("%s-tfstate-%s", f.orgSlug, env)
		fmt.Printf("ensuring TF-state bucket %s in %s\n", bucket, f.region)
		if err := client.CreateBucket(bucket, f.region); err != nil {
			return fmt.Errorf("bucket %s: %w", bucket, err)
		}
	}
	return nil
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
