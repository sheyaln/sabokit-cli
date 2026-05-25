package project

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

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
