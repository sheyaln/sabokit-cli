package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sheyaln/sabokit-cli/internal/configtf"
	"github.com/sheyaln/sabokit-cli/internal/docker"
	"github.com/sheyaln/sabokit-cli/internal/scw"
	"github.com/sheyaln/sabokit-cli/internal/template"
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

  for each env:
    cp -r environments/_template environments/<env>
    config.tf       generated from config.tf.example with prompted values
    backend.hcl     generated; bucket = "<org-slug>-tfstate-<env>"
    inventory.ini   skipped — sabokit up regenerates from terraform output
  creates the scaleway object bucket each env's backend.hcl points at
    (skip with --skip-bucket; requires SCW_ACCESS_KEY/SCW_SECRET_KEY).

after init, the only manual step before deploy is editing the generated
config.tf if you need fields beyond what was prompted (compute_hosts,
identity.tier_slots, apps, etc.) — defaults from consumer-template's
example file are preserved otherwise. then 'sabokit up'.

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
	cmd.Flags().StringVar(&f.templateTag, "from-template-tag", template.DefaultTag, "tag/branch to clone")
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

	for _, env := range envs {
		if err := scaffoldEnv(target, env, f); err != nil {
			return fmt.Errorf("scaffold env %s: %w", env, err)
		}
		fmt.Printf("scaffolded environments/%s/ (config.tf, backend.hcl)\n", env)
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
	if _, err := writeConfigYAML(target, configInputs); err != nil {
		return err
	}

	if !f.skipBucket {
		if err := ensureStateBuckets(envs, f); err != nil {
			return fmt.Errorf("create state buckets: %w", err)
		}
	}

	fmt.Printf("\ndone. next: cd %s && sabokit up\n", projectName)
	if !f.skipBucket {
		fmt.Println("    (config.tf has sensible defaults from consumer-template; edit if you need")
		fmt.Println("     compute_hosts, identity tier_slots, or app-specific overrides)")
	}
	return nil
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
		ans := prompt(r, "also scaffold a staging env? [y/N]: ", "n")
		if strings.HasPrefix(strings.ToLower(ans), "y") {
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
// materialises config.tf + backend.hcl with prompted values substituted
// in. inventory.ini is left as-is (the .example file) since `sabokit up`
// regenerates inventory.ini from terraform output anyway.
func scaffoldEnv(projectRoot, envName string, f *initFlags) error {
	if strings.ContainsAny(envName, "/\\") {
		return fmt.Errorf("env name must not contain path separators: %q", envName)
	}
	src := filepath.Join(projectRoot, "environments", "_template")
	dst := filepath.Join(projectRoot, "environments", envName)
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("environments/%s already exists", envName)
	}
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("environments/_template not found in template (tag %s)", template.DefaultTag)
	}
	if err := template.CopyTree(src, dst); err != nil {
		return err
	}

	if err := materialiseConfigTF(dst, envName, f); err != nil {
		return err
	}
	if err := materialiseBackendHCL(dst, envName, f); err != nil {
		return err
	}
	return nil
}

func materialiseConfigTF(envDir, envName string, f *initFlags) error {
	examplePath := filepath.Join(envDir, "config.tf.example")
	raw, err := os.ReadFile(examplePath)
	if err != nil {
		return fmt.Errorf("read config.tf.example: %w", err)
	}
	content := string(raw)
	substitutions := []struct{ key, value string }{
		{"scaleway_project_id", f.scwProjectID},
		{"scaleway_region", f.region},
		{"scaleway_zone", f.zone},
		{"org_slug", f.orgSlug},
		{"org_name", f.orgName},
		{"environment", envName},
		{"base_domain", f.baseDomain},
		{"gateway_domain", f.gatewayDomain},
		{"infra_email", f.infraEmail},
	}
	for _, s := range substitutions {
		if s.value == "" {
			continue
		}
		updated, ok := configtf.SetString(content, s.key, s.value)
		if !ok {
			return fmt.Errorf("config.tf.example: key %q not found (template may have shifted)", s.key)
		}
		content = updated
	}
	return os.WriteFile(filepath.Join(envDir, "config.tf"), []byte(content), 0o644)
}

func materialiseBackendHCL(envDir, envName string, f *initFlags) error {
	bucket := fmt.Sprintf("%s-tfstate-%s", f.orgSlug, envName)
	content := fmt.Sprintf(`# Generated by sabokit init. Bucket is created in the same step; key is
# the canonical layout. Override the bucket name only if a naming collision
# forces you to (S3 namespace is global).

bucket = %q
key    = "stack/terraform.tfstate"
`, bucket)
	return os.WriteFile(filepath.Join(envDir, "backend.hcl"), []byte(content), 0o644)
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
