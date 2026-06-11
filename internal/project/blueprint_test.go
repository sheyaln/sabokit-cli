package project

import (
	"os"
	"path/filepath"
	"testing"
)

// newEnv writes a project skeleton with one env's main.tf and returns the
// *Project plus the env name.
func newEnv(t *testing.T, mainTF string, vendored map[string]string) *Project {
	t.Helper()
	root := t.TempDir()
	envDir := filepath.Join(root, "environments", "prod")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(envDir, "main.tf"), []byte(mainTF), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, body := range vendored {
		p := filepath.Join(root, "modules", "stack", name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return &Project{Root: root, Config: Config{DefaultEnv: "prod"}}
}

func TestBlueprintVersion_Vendored(t *testing.T) {
	p := newEnv(t,
		`module "stack" {
  source = "../../modules/stack"
}`,
		map[string]string{
			"base.tf":     `module "base" { source = "git::https://github.com/sheyaln/sabokit.git//modules/base?ref=v0.1.0" }`,
			"identity.tf": `module "identity" { source = "git::https://github.com/sheyaln/sabokit.git//modules/identity?ref=v0.1.0" }`,
			"versions.tf": `terraform { required_version = ">= 1.9" }`,
		})
	got, err := p.BlueprintVersion("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "v0.1.0" {
		t.Errorf("got %q, want v0.1.0", got)
	}
}

func TestBlueprintVersion_RemotePin(t *testing.T) {
	p := newEnv(t,
		`module "stack" {
  source = "git::https://github.com/sheyaln/sabokit.git//consumer-template/modules/stack?ref=v0.2.3"
}`, nil)
	got, err := p.BlueprintVersion("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "v0.2.3" {
		t.Errorf("got %q, want v0.2.3", got)
	}
}

func TestBlueprintVersion_Ambiguous(t *testing.T) {
	p := newEnv(t,
		`module "stack" { source = "../../modules/stack" }`,
		map[string]string{
			"base.tf":     `source = "git::x//modules/base?ref=v0.1.0"`,
			"identity.tf": `source = "git::x//modules/identity?ref=v0.2.0"`,
		})
	if _, err := p.BlueprintVersion(""); err == nil {
		t.Fatal("want ambiguous-version error, got nil")
	}
}

func TestBlueprintVersion_NoStackBlock(t *testing.T) {
	p := newEnv(t, `module "other" { source = "./x" }`, nil)
	if _, err := p.BlueprintVersion(""); err == nil {
		t.Fatal("want error for missing stack block, got nil")
	}
}

// newFourLayerEnv writes a four-layer env: env.yml + per-layer stack.tf
// pinned at the given refs (keyed by layer; missing keys default to ref).
func newFourLayerEnv(t *testing.T, ref string, overrides map[string]string) *Project {
	t.Helper()
	root := t.TempDir()
	envDir := filepath.Join(root, "environments", "prod")
	for _, layer := range Layers {
		dir := filepath.Join(envDir, layer)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		r := ref
		if o, ok := overrides[layer]; ok {
			r = o
		}
		stack := `module "` + layer + `" { source = "git::https://github.com/sheyaln/sabokit.git//platform/` + layer + `/terraform?ref=` + r + `" }`
		if err := os.WriteFile(filepath.Join(dir, "stack.tf"), []byte(stack), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(envDir, "env.yml"), []byte("base_domain: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return &Project{Root: root, Config: Config{DefaultEnv: "prod"}}
}

func TestBlueprintVersion_FourLayer(t *testing.T) {
	p := newFourLayerEnv(t, "v0.2.0-beta2", nil)
	got, err := p.BlueprintVersion("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "v0.2.0-beta2" {
		t.Errorf("got %q, want v0.2.0-beta2", got)
	}
}

func TestBlueprintVersion_FourLayerMixedRefs(t *testing.T) {
	p := newFourLayerEnv(t, "v0.2.0", map[string]string{"application": "v0.2.1"})
	if _, err := p.BlueprintVersion(""); err == nil {
		t.Fatal("expected error for mixed per-layer refs")
	}
}
