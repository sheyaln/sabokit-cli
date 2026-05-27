package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderConfigYAML(t *testing.T) {
	in := configInputs{
		project:    "my-stack",
		baseDomain: "example.com",
		defaultEnv: "prod",
		region:     "fr-par",
		zone:       "fr-par-1",
		sshUser:    "root",
		sshKey:     "~/.ssh/id_ed25519",
	}
	got := renderConfigYAML(in)
	want := `project: my-stack
base_domain: example.com
default_env: prod
scaleway:
  region: fr-par
  zone: fr-par-1
ssh:
  user: root
  key: ~/.ssh/id_ed25519
`
	if got != want {
		t.Errorf("render mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderConfigYAMLOmitsEmptyDefaultEnv(t *testing.T) {
	in := configInputs{
		project:    "my-stack",
		baseDomain: "example.com",
		region:     "fr-par",
		zone:       "fr-par-1",
		sshUser:    "root",
		sshKey:     "~/.ssh/id_ed25519",
	}
	got := renderConfigYAML(in)
	if strings.Contains(got, "default_env") {
		t.Errorf("default_env should be omitted when empty:\n%s", got)
	}
}

func TestPromptConfigInputsNonInteractiveDefaults(t *testing.T) {
	in := configInputs{
		project:    "t",
		baseDomain: "example.com",
	}
	if err := promptConfigInputs(&in, false); err != nil {
		t.Fatal(err)
	}
	if in.region != "fr-par" {
		t.Errorf("region default = %q", in.region)
	}
	if in.zone != "fr-par-1" {
		t.Errorf("zone default = %q", in.zone)
	}
	if in.sshUser != "root" {
		t.Errorf("sshUser default = %q", in.sshUser)
	}
}

func TestPromptConfigInputsRequiresBaseDomain(t *testing.T) {
	in := configInputs{project: "t"}
	err := promptConfigInputs(&in, false)
	if err == nil {
		t.Fatal("expected error for missing base_domain")
	}
}

func TestWriteConfigYAML(t *testing.T) {
	dir := t.TempDir()
	in := configInputs{
		project:    "t",
		baseDomain: "example.com",
		region:     "fr-par",
		zone:       "fr-par-1",
		sshUser:    "root",
		sshKey:     "~/.ssh/id_ed25519",
	}
	path, err := writeConfigYAML(dir, in)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, ".sabokit", "config.yml")
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "project: t") {
		t.Errorf("missing project: t in:\n%s", raw)
	}
}
