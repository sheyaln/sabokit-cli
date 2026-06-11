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

func TestEnabledAppsPath(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "environments", "prod")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := &Project{Root: dir, Config: Config{DefaultEnv: "prod"}}

	want := filepath.Join(envDir, ".enabled_apps.json")
	if got := p.EnabledAppsPath(""); got != want {
		t.Errorf("EnabledAppsPath = %q, want %q", got, want)
	}

	pNoEnv := &Project{Root: dir, Config: Config{}}
	if got := pNoEnv.EnabledAppsPath(""); got != "" {
		t.Errorf("no env EnabledAppsPath = %q, want empty", got)
	}
}

func TestIsFourLayer(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "environments", "prod")
	if err := os.MkdirAll(filepath.Join(envDir, "infra"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := &Project{Root: dir, Config: Config{DefaultEnv: "prod"}}
	if p.IsFourLayer("") {
		t.Error("env.yml + infra/stack.tf absent → not four-layer")
	}
	if err := os.WriteFile(filepath.Join(envDir, "env.yml"), []byte("base_domain: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(envDir, "infra", "stack.tf"), []byte("# stack"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !p.IsFourLayer("") {
		t.Error("env.yml + infra/stack.tf present → four-layer")
	}
}

func TestLoadAndCatalog(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, configDir), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `project: test
scaleway:
  region: fr-par
ssh:
  user: root
`
	manifest := `schema_version: 1
apps:
  - id: outline
    display_name: Outline
    category: collaboration
    description_short: Team wiki
  - id: espocrm
    display_name: EspoCRM
    category: crm
    description_short: Open source CRM
  - id: n8n
    display_name: n8n
    category: automation
    description_short: Workflow automation
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
	apps, err := p.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 3 {
		t.Fatalf("got %d apps, want 3", len(apps))
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].ID < apps[j].ID })
	if apps[0].ID != "espocrm" || apps[0].DisplayName != "EspoCRM" {
		t.Errorf("espocrm entry wrong: %+v", apps[0])
	}
	if apps[2].Category != "collaboration" {
		t.Errorf("outline category = %q", apps[2].Category)
	}
}

func TestEnvAppsFromEnabledAppsJSON(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "environments", "prod")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `schema_version: 1
apps:
  - id: outline
    display_name: Outline
    category: collaboration
  - id: espocrm
    display_name: EspoCRM
    category: crm
  - id: n8n
    display_name: n8n
    category: automation
`
	vars := `{
		"enabled_apps": {
			"outline": {"url": "https://wiki.example.com"},
			"espocrm": {"url": "https://crm.example.com"},
			"n8n": null
		}
	}`
	if err := os.WriteFile(filepath.Join(dir, "apps-manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(envDir, ".enabled_apps.json"), []byte(vars), 0o644); err != nil {
		t.Fatal(err)
	}

	p := &Project{Root: dir, Config: Config{Inventory: "inventory.ini", AppsManifest: "apps-manifest.yaml", DefaultEnv: "prod"}}
	apps, err := p.EnvApps("")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]EnvApp{}
	for _, a := range apps {
		got[a.ID] = a
	}
	if !got["outline"].Enabled || got["outline"].URL != "https://wiki.example.com" {
		t.Errorf("outline = %+v", got["outline"])
	}
	if !got["espocrm"].Enabled {
		t.Errorf("espocrm should be enabled: %+v", got["espocrm"])
	}
	if got["n8n"].Enabled {
		t.Errorf("n8n should be disabled (null value): %+v", got["n8n"])
	}
}

func TestInventoryHosts(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "environments", "prod")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}
	inv := `[apps]
app01-prod ansible_host=51.1.1.1 ansible_user=root
app02-prod ansible_host=51.1.1.2 ansible_user=root

[identity]
auth01-prod ansible_host=51.2.2.2 ansible_user=root

[all:vars]
ansible_python_interpreter=/usr/bin/python3
`
	if err := os.WriteFile(filepath.Join(envDir, "inventory.ini"), []byte(inv), 0o644); err != nil {
		t.Fatal(err)
	}

	p := &Project{Root: dir, Config: Config{Inventory: "inventory.ini", DefaultEnv: "prod"}}

	apps, err := p.InventoryHosts("", "apps")
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 2 || apps[0] != "app01-prod" || apps[1] != "app02-prod" {
		t.Errorf("apps hosts = %v", apps)
	}
	identity, err := p.InventoryHosts("", "identity")
	if err != nil {
		t.Fatal(err)
	}
	if len(identity) != 1 || identity[0] != "auth01-prod" {
		t.Errorf("identity hosts = %v", identity)
	}
	none, err := p.InventoryHosts("", "missing")
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Errorf("missing group should be empty, got %v", none)
	}
}
