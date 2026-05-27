package project

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestWorkspaceDir(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "environments", "prod")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}

	p := &Project{Root: dir, Config: Config{}}

	// no env set → root
	got, err := p.WorkspaceDir("")
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Errorf("no env: got %q, want %q", got, dir)
	}

	// env override
	got, err = p.WorkspaceDir("prod")
	if err != nil {
		t.Fatal(err)
	}
	if got != envDir {
		t.Errorf("override: got %q, want %q", got, envDir)
	}

	// default_env in config
	p2 := &Project{Root: dir, Config: Config{DefaultEnv: "prod"}}
	got, err = p2.WorkspaceDir("")
	if err != nil {
		t.Fatal(err)
	}
	if got != envDir {
		t.Errorf("default_env: got %q, want %q", got, envDir)
	}

	// override beats default
	got, err = p2.WorkspaceDir("staging")
	if err == nil {
		t.Errorf("expected error for missing env, got %q", got)
	}

	// missing env errors
	_, err = p.WorkspaceDir("missing")
	if err == nil {
		t.Error("expected error for missing env dir")
	}
}

func TestAnsibleVarsAndTFOutputPaths(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "environments", "prod")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := &Project{Root: dir, Config: Config{DefaultEnv: "prod"}}

	want := filepath.Join(envDir, ".ansible-vars.json")
	if got := p.AnsibleVarsPath(""); got != want {
		t.Errorf("AnsibleVarsPath = %q, want %q", got, want)
	}
	want = filepath.Join(envDir, ".tf-output.json")
	if got := p.TFOutputPath(""); got != want {
		t.Errorf("TFOutputPath = %q, want %q", got, want)
	}

	pNoEnv := &Project{Root: dir, Config: Config{}}
	if got := pNoEnv.AnsibleVarsPath(""); got != "" {
		t.Errorf("no env AnsibleVarsPath = %q, want empty", got)
	}
	if got := pNoEnv.TFOutputPath(""); got != "" {
		t.Errorf("no env TFOutputPath = %q, want empty", got)
	}
}

func TestLoadAndEnabledApps(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, configDir), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `project: test
scaleway:
  region: fr-par
  zone: fr-par-1
ssh:
  user: root
  key: ~/.ssh/id_ed25519
`
	manifest := `apps:
  espocrm:
    enabled: true
    host: app01
  n8n:
    enabled: false
    host: app01
  authentik:
    enabled: true
    host: app02
`
	if err := os.WriteFile(filepath.Join(dir, configDir, configFile), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "apps-manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	prevCwd, _ := os.Getwd()
	defer os.Chdir(prevCwd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if p.Config.Project != "test" {
		t.Errorf("project = %q, want %q", p.Config.Project, "test")
	}
	if p.Config.Scaleway.Region != "fr-par" {
		t.Errorf("region = %q", p.Config.Scaleway.Region)
	}
	if p.Config.Inventory != "inventory.ini" {
		t.Errorf("inventory default not applied: %q", p.Config.Inventory)
	}

	apps, err := p.EnabledApps()
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(apps)
	want := []string{"authentik", "espocrm"}
	if len(apps) != len(want) {
		t.Fatalf("apps = %v, want %v", apps, want)
	}
	for i := range apps {
		if apps[i] != want[i] {
			t.Fatalf("apps = %v, want %v", apps, want)
		}
	}
}
