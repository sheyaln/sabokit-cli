package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedEnvDir writes a fully-populated environments/<env>/ that mirrors
// the post-init shape: legitimate committed files plus the runtime /
// secrets / state artifacts that env-add must NOT copy across envs.
func seedEnvDir(t *testing.T, projectRoot, env string) {
	t.Helper()
	dir := filepath.Join(projectRoot, "environments", env)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"config.tf":              configTFSeed(env),
		"main.tf":                "# main\n",
		"providers.tf":           "# providers\n",
		"variables.tf":           "# vars\n",
		"secrets.tf":             "# secrets data blocks\n",
		"outputs.tf":             "# outputs\n",
		"moved.tf":               "# moved blocks\n",
		"README.md":              "# env " + env + "\n",
		"config.tf.example":      "# example\n",
		"backend.hcl.example":    `bucket = "x"` + "\n",
		"terraform.tfvars":       "secret_admin_pw = \"hunter2\"\n",
		"prod.tfvars":            "ignored\n",
		"backend.hcl":            `bucket = "old-bucket"` + "\n",
		"inventory.ini":          "[apps]\n",
		".terraform.lock.hcl":    "# lock\n",
		"terraform.tfstate":      "{}\n",
		"terraform.tfstate.backup": "{}\n",
		".envrc":                 "export X=1\n",
		".env":                   "X=1\n",
		".ansible-vars.json":     "{}\n",
		".tf-output.json":        "{}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// .terraform/ directory must be pruned (whole dir skipped).
	if err := os.MkdirAll(filepath.Join(dir, ".terraform", "modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".terraform", "modules", "junk.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func configTFSeed(env string) string {
	return `locals {
  config = {
    scaleway_project_id = "00000000-0000-0000-0000-000000000000"
    scaleway_region     = "fr-par"
    scaleway_zone       = "fr-par-1"
    org_slug    = "demo"
    org_name    = "Demo"
    environment = "` + env + `"
    base_domain    = "example.org"
    gateway_domain = "auth.example.org"
    infra_email    = "ops@example.org"
  }
}
`
}

// seedProjectRoot writes the minimal .sabokit/config.yml the env-add
// command needs to anchor a project root.
func seedProjectRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".sabokit"), 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := `project: demo
base_domain: example.org
default_env: prod
scaleway:
  region: fr-par
  zone: fr-par-1
ssh:
  user: ubuntu
  key: ~/.ssh/id_ed25519
`
	if err := os.WriteFile(filepath.Join(root, ".sabokit", "config.yml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// withCwd swaps cwd to dir for the duration of the test. project.Load
// walks up from cwd; the env-add command needs that to land inside the
// fixture root.
func withCwd(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func TestEnvAddCarbonCopiesAndSkipsCorrectly(t *testing.T) {
	root := seedProjectRoot(t)
	seedEnvDir(t, root, "prod")
	withCwd(t, root)

	if err := runEnvAdd("staging", &envAddFlags{from: "prod", skipBucket: true}); err != nil {
		t.Fatalf("env add: %v", err)
	}

	dst := filepath.Join(root, "environments", "staging")

	// Files that MUST exist post-copy (committable shared shape).
	mustExist := []string{
		"config.tf", "main.tf", "providers.tf", "variables.tf", "secrets.tf",
		"outputs.tf", "moved.tf", "README.md", "config.tf.example",
		"backend.hcl.example", "backend.hcl", // backend.hcl regenerated
	}
	for _, name := range mustExist {
		if _, err := os.Stat(filepath.Join(dst, name)); err != nil {
			t.Errorf("expected %q in copied env, got %v", name, err)
		}
	}

	// Files that MUST NOT have been copied.
	mustNotExist := []string{
		"terraform.tfvars", "prod.tfvars",
		"inventory.ini",
		".terraform.lock.hcl",
		"terraform.tfstate", "terraform.tfstate.backup",
		".envrc", ".env",
		".ansible-vars.json", ".tf-output.json",
	}
	for _, name := range mustNotExist {
		if _, err := os.Stat(filepath.Join(dst, name)); !os.IsNotExist(err) {
			t.Errorf("file %q should NOT be copied, got err=%v", name, err)
		}
	}

	// .terraform/ directory must not exist in destination.
	if _, err := os.Stat(filepath.Join(dst, ".terraform")); !os.IsNotExist(err) {
		t.Errorf(".terraform/ should be pruned, got err=%v", err)
	}
}

func TestEnvAddRegeneratesBackendHCL(t *testing.T) {
	root := seedProjectRoot(t)
	seedEnvDir(t, root, "prod")
	withCwd(t, root)

	if err := runEnvAdd("staging", &envAddFlags{from: "prod", skipBucket: true}); err != nil {
		t.Fatalf("env add: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "environments", "staging", "backend.hcl"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.Contains(got, `bucket = "demo-tfstate-staging"`) {
		t.Errorf("backend.hcl should reference new env bucket, got:\n%s", got)
	}
	if strings.Contains(got, "old-bucket") {
		t.Errorf("backend.hcl should not retain source bucket reference, got:\n%s", got)
	}
}

func TestEnvAddSubstitutesEnvironmentString(t *testing.T) {
	root := seedProjectRoot(t)
	seedEnvDir(t, root, "prod")
	withCwd(t, root)

	if err := runEnvAdd("staging", &envAddFlags{from: "prod", skipBucket: true}); err != nil {
		t.Fatalf("env add: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "environments", "staging", "config.tf"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.Contains(got, `environment = "staging"`) {
		t.Errorf("config.tf should have environment = \"staging\":\n%s", got)
	}
	if strings.Contains(got, `environment = "prod"`) {
		t.Errorf("config.tf still has source env string:\n%s", got)
	}
}

func TestEnvAddDoesNotTouchUnrelatedOccurrencesOfSourceEnvName(t *testing.T) {
	root := seedProjectRoot(t)
	dir := filepath.Join(root, "environments", "prod")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// config.tf where the source env name "prod" also appears in unrelated
	// places (a domain, a comment). Only the precise HCL assignment should
	// be rewritten.
	body := `locals {
  config = {
    org_slug    = "demo"
    environment = "prod"
    base_domain = "prod.example.org"  # production domain
  }
}
`
	if err := os.WriteFile(filepath.Join(dir, "config.tf"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	withCwd(t, root)

	if err := runEnvAdd("staging", &envAddFlags{from: "prod", skipBucket: true}); err != nil {
		t.Fatalf("env add: %v", err)
	}
	out, _ := os.ReadFile(filepath.Join(root, "environments", "staging", "config.tf"))
	got := string(out)
	if !strings.Contains(got, `environment = "staging"`) {
		t.Errorf("environment line not rewritten:\n%s", got)
	}
	if !strings.Contains(got, `prod.example.org`) {
		t.Errorf("unrelated occurrence of 'prod' in domain should be untouched:\n%s", got)
	}
	if !strings.Contains(got, `# production domain`) {
		t.Errorf("unrelated comment should be untouched:\n%s", got)
	}
}

func TestEnvAddValidationErrors(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(t *testing.T, root string)
		newName   string
		fromEnv   string
		wantMatch string
	}{
		{
			name:      "source env missing",
			setup:     func(t *testing.T, root string) {},
			newName:   "staging",
			fromEnv:   "prod",
			wantMatch: "source env not found",
		},
		{
			name: "new env already exists",
			setup: func(t *testing.T, root string) {
				seedEnvDir(t, root, "prod")
				seedEnvDir(t, root, "staging")
			},
			newName:   "staging",
			fromEnv:   "prod",
			wantMatch: "already exists",
		},
		{
			name:      "invalid name uppercase",
			setup:     func(t *testing.T, root string) { seedEnvDir(t, root, "prod") },
			newName:   "STAGING",
			fromEnv:   "prod",
			wantMatch: "invalid env name",
		},
		{
			name:      "invalid name path separator",
			setup:     func(t *testing.T, root string) { seedEnvDir(t, root, "prod") },
			newName:   "stag/ing",
			fromEnv:   "prod",
			wantMatch: "invalid env name",
		},
		{
			name:      "name equals source",
			setup:     func(t *testing.T, root string) { seedEnvDir(t, root, "prod") },
			newName:   "prod",
			fromEnv:   "prod",
			wantMatch: "must differ",
		},
		{
			name:      "name too long",
			setup:     func(t *testing.T, root string) { seedEnvDir(t, root, "prod") },
			newName:   strings.Repeat("a", 31),
			fromEnv:   "prod",
			wantMatch: "invalid env name",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := seedProjectRoot(t)
			tc.setup(t, root)
			withCwd(t, root)
			err := runEnvAdd(tc.newName, &envAddFlags{from: tc.fromEnv, skipBucket: true})
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantMatch)
			}
			if !strings.Contains(err.Error(), tc.wantMatch) {
				t.Errorf("expected error containing %q, got: %v", tc.wantMatch, err)
			}
		})
	}
}

func TestEnvAddSkipBucketHonored(t *testing.T) {
	// When --skip-bucket is set, the command must not require SCW_* creds
	// and must succeed even when docker isn't reachable. We assert success
	// here without setting any creds — proves we never reached the bucket
	// codepath.
	root := seedProjectRoot(t)
	seedEnvDir(t, root, "prod")
	withCwd(t, root)
	t.Setenv("SCW_ACCESS_KEY", "")
	t.Setenv("SCW_SECRET_KEY", "")

	if err := runEnvAdd("staging", &envAddFlags{from: "prod", skipBucket: true}); err != nil {
		t.Fatalf("--skip-bucket should bypass SCW preflight, got: %v", err)
	}
}

func TestShouldSkipEnvFile(t *testing.T) {
	skip := []string{
		"terraform.tfvars", "prod.tfvars", "backend.hcl", "inventory.ini",
		".terraform.lock.hcl", ".envrc", ".env",
		"terraform.tfstate", "terraform.tfstate.backup",
		".ansible-vars.json", ".tf-output.json",
	}
	keep := []string{
		"config.tf", "main.tf", "providers.tf", "secrets.tf", "outputs.tf",
		"moved.tf", "README.md", "config.tf.example", "backend.hcl.example",
		"prod.tfvars.example",
	}
	for _, n := range skip {
		if !shouldSkipEnvFile(n) {
			t.Errorf("%q should be skipped", n)
		}
	}
	for _, n := range keep {
		if shouldSkipEnvFile(n) {
			t.Errorf("%q should be kept", n)
		}
	}
}

func TestValidateEnvName(t *testing.T) {
	good := []string{"prod", "staging", "dev1", "ephemeral-pr-42", "a"}
	bad := []string{"", "PROD", "stag ing", "stag/ing", "-leading", strings.Repeat("a", 31)}
	for _, n := range good {
		if err := validateEnvName(n); err != nil {
			t.Errorf("%q should be valid, got: %v", n, err)
		}
	}
	for _, n := range bad {
		if err := validateEnvName(n); err == nil {
			t.Errorf("%q should be invalid", n)
		}
	}
}
