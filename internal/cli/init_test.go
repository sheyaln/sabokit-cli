package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeTemplate writes a minimal environments/_template into projectRoot
// that mirrors the shape of upstream consumer-template's _template dir:
// the legitimate TF files + the dead artifacts sabokit-cli must strip.
func fakeTemplate(t *testing.T, projectRoot string) {
	t.Helper()
	dir := filepath.Join(projectRoot, "environments", "_template")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"main.tf":                "# main",
		"providers.tf":           "# providers",
		"variables.tf":           "# vars",
		"secrets.tf":             "# secrets",
		"backend.hcl.example":    `bucket = "x"`,
		"config.tf.example":      configTFExampleFixture(),
		"README.md":              "# README",
		"inventory.ini.example":  "[apps]\n",
		"preflight.sh":           "#!/bin/sh\n",
		"up.sh":                  "#!/bin/sh\n",
		"configure.sh":           "#!/bin/sh\n",
		"_lib.sh":                "# lib",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func configTFExampleFixture() string {
	return `locals {
  config = {
    scaleway_project_id = "00000000-0000-0000-0000-000000000000"
    scaleway_region     = "fr-par"
    scaleway_zone       = "fr-par-1"
    org_slug    = "acme"
    org_name    = "Acme"
    environment = "prod"
    base_domain    = "example.org"
    gateway_domain = "auth.example.org"
    infra_email    = "ops@example.org"
  }
}
`
}

func TestScaffoldEnvStripsDeadArtifacts(t *testing.T) {
	root := t.TempDir()
	fakeTemplate(t, root)

	f := &initFlags{
		baseDomain:    "example.org",
		gatewayDomain: "auth.example.org",
		scwProjectID:  "abc-123",
		orgSlug:       "acme",
		orgName:       "Acme",
		infraEmail:    "ops@example.org",
		region:        "fr-par",
		zone:          "fr-par-1",
	}
	if err := scaffoldEnv(root, "prod", f); err != nil {
		t.Fatal(err)
	}
	envDir := filepath.Join(root, "environments", "prod")

	for _, dead := range deadEnvArtifacts {
		if _, err := os.Stat(filepath.Join(envDir, dead)); !os.IsNotExist(err) {
			t.Errorf("dead artifact %q should have been removed", dead)
		}
	}
	// Legitimate files survive.
	keep := []string{"main.tf", "providers.tf", "variables.tf", "secrets.tf", "README.md", "backend.hcl.example", "config.tf.example"}
	for _, k := range keep {
		if _, err := os.Stat(filepath.Join(envDir, k)); err != nil {
			t.Errorf("expected %q to survive scaffold, got %v", k, err)
		}
	}
	// Generated files exist.
	for _, gen := range []string{"config.tf", "backend.hcl"} {
		if _, err := os.Stat(filepath.Join(envDir, gen)); err != nil {
			t.Errorf("expected generated %q, got %v", gen, err)
		}
	}
}

func TestScaffoldEnvSubstitutesConfigTF(t *testing.T) {
	root := t.TempDir()
	fakeTemplate(t, root)
	f := &initFlags{
		baseDomain:    "real.test",
		gatewayDomain: "auth.real.test",
		scwProjectID:  "deadbeef-1111-2222-3333-444455556666",
		orgSlug:       "neworg",
		orgName:       "New Org",
		infraEmail:    "ops@real.test",
		region:        "nl-ams",
		zone:          "nl-ams-2",
	}
	if err := scaffoldEnv(root, "staging", f); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "environments", "staging", "config.tf"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	want := map[string]string{
		"scaleway_project_id": `"deadbeef-1111-2222-3333-444455556666"`,
		"scaleway_region":     `"nl-ams"`,
		"scaleway_zone":       `"nl-ams-2"`,
		"org_slug":            `"neworg"`,
		"org_name":            `"New Org"`,
		"environment":         `"staging"`,
		"base_domain":         `"real.test"`,
		"gateway_domain":      `"auth.real.test"`,
		"infra_email":         `"ops@real.test"`,
	}
	for key, val := range want {
		if !strings.Contains(got, val) {
			t.Errorf("config.tf missing %s = %s\n%s", key, val, got)
		}
	}
}

func TestScaffoldEnvBackendHCLBucket(t *testing.T) {
	root := t.TempDir()
	fakeTemplate(t, root)
	f := &initFlags{orgSlug: "demo", baseDomain: "x.org", gatewayDomain: "auth.x.org",
		scwProjectID: "1", orgName: "D", infraEmail: "a@x", region: "fr-par", zone: "fr-par-1"}
	if err := scaffoldEnv(root, "prod", f); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "environments", "prod", "backend.hcl"))
	if !strings.Contains(string(body), `bucket = "demo-tfstate-prod"`) {
		t.Errorf("backend.hcl missing expected bucket name:\n%s", body)
	}
}

func TestResolveEnvsCombinesEnvAndStaging(t *testing.T) {
	cases := []struct {
		name    string
		f       initFlags
		want    []string
	}{
		{"explicit env only", initFlags{envs: []string{"prod"}}, []string{"prod"}},
		{"explicit envs list", initFlags{envs: []string{"prod", "staging"}}, []string{"prod", "staging"}},
		{"staging flag alone (non-interactive)", initFlags{staging: true, nonInteractive: true}, []string{"prod", "staging"}},
		{"env=prod + staging flag", initFlags{envs: []string{"prod"}, staging: true, nonInteractive: true}, []string{"prod", "staging"}},
		{"non-interactive default", initFlags{nonInteractive: true}, []string{"prod"}},
	}
	for _, tc := range cases {
		got := resolveEnvs(&tc.f)
		if len(got) != len(tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
				break
			}
		}
	}
}

func TestValidateBucketNamesEnforcesS3Limit(t *testing.T) {
	short := &initFlags{orgSlug: "acme"}
	if err := validateBucketNames([]string{"prod"}, short); err != nil {
		t.Errorf("short slug should pass: %v", err)
	}
	long := &initFlags{orgSlug: strings.Repeat("x", 60)}
	if err := validateBucketNames([]string{"prod"}, long); err == nil {
		t.Error("60-char slug should overflow 63-char limit")
	}
	skipped := &initFlags{orgSlug: strings.Repeat("x", 100), skipBucket: true}
	if err := validateBucketNames([]string{"prod"}, skipped); err != nil {
		t.Errorf("--skip-bucket should bypass the precheck: %v", err)
	}
}

func TestWriteGitignoreIncludesTFVars(t *testing.T) {
	dir := t.TempDir()
	if err := writeGitignore(dir); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	for _, must := range []string{"*.tfvars", "*.tfstate.backup", "*.tfplan", ".terraform/", ".tf-output.json"} {
		if !strings.Contains(string(body), must) {
			t.Errorf(".gitignore missing %q:\n%s", must, body)
		}
	}
	if !strings.Contains(string(body), "!*.tfvars.example") {
		t.Errorf(".gitignore should keep .tfvars.example tracked")
	}
}

func TestWriteEnvrcExampleHasSCWVars(t *testing.T) {
	dir := t.TempDir()
	if err := writeEnvrcExample(dir); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".envrc.example"))
	for _, must := range []string{"SCW_ACCESS_KEY", "SCW_SECRET_KEY", "SCW_DEFAULT_PROJECT_ID", "SABOKIT_PLATFORM"} {
		if !strings.Contains(string(body), must) {
			t.Errorf(".envrc.example missing %q", must)
		}
	}
}

func TestWriteOperatorREADMEMentionsEnvs(t *testing.T) {
	dir := t.TempDir()
	if err := writeOperatorREADME(dir, "my-stack", []string{"prod", "staging"}, &initFlags{orgSlug: "demo"}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "README.md"))
	got := string(body)
	if !strings.Contains(got, "# my-stack") {
		t.Errorf("missing project title")
	}
	if !strings.Contains(got, "`prod`") || !strings.Contains(got, "`staging`") {
		t.Errorf("missing env listing:\n%s", got)
	}
	if !strings.Contains(got, "demo-tfstate-<env>") {
		t.Errorf("missing bucket pattern")
	}
}
