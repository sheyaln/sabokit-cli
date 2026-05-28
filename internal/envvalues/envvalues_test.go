package envvalues

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeEnvValues(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "environments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "env-values.yml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

const sample = `
prod:
  scaleway_project_id: "proj-prod"
  base_domain: "example.org"
  identity_domain: "auth.example.org"
  infra_email: "ops@example.org"
  compute_instance_types:
    tools: "DEV1-L"
staging:
  scaleway_project_id: "proj-staging"
  base_domain: "staging.example.org"
  identity_domain: "auth.staging.example.org"
  infra_email: "ops@example.org"
`

func TestGetAndString(t *testing.T) {
	root := writeEnvValues(t, sample)
	s, err := Get(root, "prod")
	if err != nil {
		t.Fatal(err)
	}
	if got := s.String("scaleway_project_id"); got != "proj-prod" {
		t.Errorf("project_id = %q, want proj-prod", got)
	}
	if got := s.String("base_domain"); got != "example.org" {
		t.Errorf("base_domain = %q", got)
	}
	// nested object must not stringify
	if got := s.String("compute_instance_types"); got != "" {
		t.Errorf("nested map stringified to %q, want empty", got)
	}
}

func TestGetMissingEnv(t *testing.T) {
	root := writeEnvValues(t, sample)
	_, err := Get(root, "dev")
	if err == nil {
		t.Fatal("expected error for missing env")
	}
	if !strings.Contains(err.Error(), "prod") || !strings.Contains(err.Error(), "staging") {
		t.Errorf("error should list available envs, got: %v", err)
	}
}

func TestRequire(t *testing.T) {
	root := writeEnvValues(t, sample)
	s, _ := Get(root, "prod")
	if err := s.Require(RequiredKeys...); err != nil {
		t.Errorf("prod should satisfy required keys: %v", err)
	}

	bad := writeEnvValues(t, "prod:\n  base_domain: \"x\"\n")
	s2, _ := Get(bad, "prod")
	if err := s2.Require(RequiredKeys...); err == nil {
		t.Fatal("expected missing-key error")
	}
}

func TestCheckDistinctProjectIDs(t *testing.T) {
	root := writeEnvValues(t, sample)
	all, _ := Load(root)
	if err := CheckDistinctProjectIDs(all); err != nil {
		t.Errorf("distinct project ids should pass: %v", err)
	}

	dup := writeEnvValues(t, "prod:\n  scaleway_project_id: \"same\"\nstaging:\n  scaleway_project_id: \"same\"\n")
	all2, _ := Load(dup)
	if err := CheckDistinctProjectIDs(all2); err == nil {
		t.Fatal("expected duplicate project_id error")
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("expected error for missing env-values.yml")
	}
}
