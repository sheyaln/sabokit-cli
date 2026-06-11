package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sheyaln/sabokit-cli/internal/project"
)

// fakeTemplate writes a minimal four-layer environments/_template (plus
// environments/common.yml) into projectRoot, mirroring upstream
// consumer-template's shape.
func fakeTemplate(t *testing.T, projectRoot string) {
	t.Helper()
	dir := filepath.Join(projectRoot, "environments", "_template")
	for _, layer := range project.Layers {
		if err := os.MkdirAll(filepath.Join(dir, layer), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"env.yml":         envYMLTemplateFixture(),
		"hosts.yml":       "compute_hosts:\n  tools:\n    role: apps\n",
		"infra.yml":       "postgres_enabled: true\n",
		"identity.yml":    "tier_slots: []\n",
		"operations.yml":  "grafana:\n  enabled: true\n",
		"application.yml": "outline:\n  enabled: true\n  hostname: wiki.example.org\n",
		"README.md":       "# env",
	}
	for _, layer := range project.Layers {
		files[layer+"/stack.tf"] = "module \"" + layer + "\" {\n  source = \"git::https://github.com/sheyaln/sabokit.git//platform/" + layer + "/terraform?ref=v0.2.0\"\n}\n"
		files[layer+"/providers.tf"] = "# providers"
		files[layer+"/backend.hcl.example"] = `bucket = "acme-tfstate-prod"` + "\n" + `key = "` + layer + `/terraform.tfstate"` + "\n"
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	common := "org_slug: acme\norg_name: Acme Cooperative\n"
	if err := os.WriteFile(filepath.Join(projectRoot, "environments", "common.yml"), []byte(common), 0o644); err != nil {
		t.Fatal(err)
	}
}

func envYMLTemplateFixture() string {
	return `# Per-env identity + sizing.
scaleway_project_id: "00000000-0000-0000-0000-000000000000" # replace
scaleway_region: fr-par
scaleway_zone: fr-par-1

base_domain: example.org
mgmt_domain: ""
identity_domain: auth.example.org
infra_email: ops@example.org

private_network_subnet: 10.0.0.0/22
compute_instance_types:
  tools: DEV1-L
`
}

func testInitFlags() *initFlags {
	return &initFlags{
		baseDomain:    "real.test",
		gatewayDomain: "auth.real.test",
		scwProjectID:  "deadbeef-1111-2222-3333-444455556666",
		orgSlug:       "neworg",
		orgName:       "New Org",
		infraEmail:    "ops@real.test",
		region:        "nl-ams",
		zone:          "nl-ams-2",
	}
}

func TestScaffoldEnvMaterialisesEnvYML(t *testing.T) {
	root := t.TempDir()
	fakeTemplate(t, root)
	if err := scaffoldEnv(root, "prod", testInitFlags(), true); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "environments", "prod", "env.yml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	for _, must := range []string{
		`scaleway_project_id: "deadbeef-1111-2222-3333-444455556666"`,
		`base_domain: "real.test"`,
		`identity_domain: "auth.real.test"`,
		`infra_email: "ops@real.test"`,
		`scaleway_region: "nl-ams"`,
		`scaleway_zone: "nl-ams-2"`,
	} {
		if !strings.Contains(got, must) {
			t.Errorf("env.yml missing %q:\n%s", must, got)
		}
	}
	// untouched keys + comments survive
	if !strings.Contains(got, "private_network_subnet: 10.0.0.0/22") {
		t.Errorf("untouched key lost:\n%s", got)
	}
	if !strings.Contains(got, "# Per-env identity + sizing.") {
		t.Errorf("comments lost:\n%s", got)
	}
}

func TestScaffoldEnvSecondaryGetsPlaceholderProjectID(t *testing.T) {
	root := t.TempDir()
	fakeTemplate(t, root)
	f := testInitFlags()
	if err := scaffoldEnv(root, "prod", f, true); err != nil {
		t.Fatal(err)
	}
	if err := scaffoldEnv(root, "staging", f, false); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "environments", "staging", "env.yml"))
	if strings.Contains(string(body), "deadbeef-1111") {
		t.Errorf("staging must not reuse prod's project_id:\n%s", body)
	}
	if !strings.Contains(string(body), "REPLACE-with-staging-project-uuid") {
		t.Errorf("staging should get a placeholder project_id:\n%s", body)
	}
}

func TestScaffoldEnvWritesPerLayerBackends(t *testing.T) {
	root := t.TempDir()
	fakeTemplate(t, root)
	f := testInitFlags()
	f.orgSlug = "demo"
	if err := scaffoldEnv(root, "prod", f, true); err != nil {
		t.Fatal(err)
	}
	for _, layer := range project.Layers {
		body, err := os.ReadFile(filepath.Join(root, "environments", "prod", layer, "backend.hcl"))
		if err != nil {
			t.Fatalf("missing %s/backend.hcl: %v", layer, err)
		}
		if !strings.Contains(string(body), `bucket = "demo-tfstate-prod"`) {
			t.Errorf("%s/backend.hcl missing bucket:\n%s", layer, body)
		}
		if !strings.Contains(string(body), `key    = "`+layer+`/terraform.tfstate"`) {
			t.Errorf("%s/backend.hcl missing per-layer key:\n%s", layer, body)
		}
	}
}

func TestMaterialiseCommonYML(t *testing.T) {
	root := t.TempDir()
	fakeTemplate(t, root)
	if err := materialiseCommonYML(root, testInitFlags()); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "environments", "common.yml"))
	got := string(body)
	if !strings.Contains(got, `org_slug: "neworg"`) {
		t.Errorf("org_slug not substituted:\n%s", got)
	}
	if !strings.Contains(got, `org_name: "New Org"`) {
		t.Errorf("org_name not substituted:\n%s", got)
	}
}

func TestResolveEnvsCombinesEnvAndStaging(t *testing.T) {
	cases := []struct {
		name string
		f    initFlags
		want []string
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

func TestWriteGitignore(t *testing.T) {
	dir := t.TempDir()
	if err := writeGitignore(dir); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	got := string(body)
	for _, must := range []string{"*.tfvars", "*.tfstate.backup", "*.tfplan", ".terraform/", ".enabled_apps.json", "inventory.ini", "backend.hcl", ".envrc"} {
		if !strings.Contains(got, must) {
			t.Errorf(".gitignore missing %q:\n%s", must, got)
		}
	}
	if !strings.Contains(got, "!.envrc.example") {
		t.Errorf(".gitignore should keep .envrc.example tracked:\n%s", got)
	}
}

func TestWriteEnvrcExampleHasSCWVars(t *testing.T) {
	dir := t.TempDir()
	if err := writeEnvrcExample(dir); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".envrc.example"))
	for _, must := range []string{"SCW_ACCESS_KEY", "SCW_SECRET_KEY", "SCW_DEFAULT_PROJECT_ID"} {
		if !strings.Contains(string(body), must) {
			t.Errorf(".envrc.example missing %q", must)
		}
	}
	// SABOKIT_PLATFORM is no longer scaffolded — the CLI defaults to the host
	// arch and every image is multi-arch.
	if strings.Contains(string(body), "SABOKIT_PLATFORM") {
		t.Errorf(".envrc.example should not pin SABOKIT_PLATFORM:\n%s", body)
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
