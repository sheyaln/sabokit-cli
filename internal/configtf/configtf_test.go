package configtf

import (
	"strings"
	"testing"
)

const sample = `locals {
  config = {
    apps = {
      outline = {
        enabled  = true
        hostname = "wiki.example.org"
      }

      # vikunja = {
      #   enabled  = true
      #   hostname = "tasks.example.org"
      # }

      espocrm = {
        enabled  = false
        hostname = "crm.example.org"
      }

      grafana = {
        hostname = "metrics.example.org"
      }
    }
  }
}
`

func TestSetString(t *testing.T) {
	in := `locals {
  config = {
    scaleway_project_id = "00000000-0000-0000-0000-000000000000" # replace with your project UUID
    org_slug    = "acme"
    base_domain = "example.org"
  }
}`
	out, ok := SetString(in, "scaleway_project_id", "abc-123")
	if !ok {
		t.Fatal("expected found=true")
	}
	if !strings.Contains(out, `scaleway_project_id = "abc-123"`) {
		t.Errorf("uuid not replaced:\n%s", out)
	}
	if !strings.Contains(out, "# replace with your project UUID") {
		t.Errorf("trailing comment lost:\n%s", out)
	}
	out, ok = SetString(out, "org_slug", "neworg")
	if !ok || !strings.Contains(out, `org_slug    = "neworg"`) {
		t.Errorf("org_slug not replaced or indentation broken:\n%s", out)
	}
	if _, ok := SetString(in, "missing_key", "x"); ok {
		t.Error("expected found=false for missing key")
	}
}

func TestGetString(t *testing.T) {
	tf := `locals {
  config = {
    scaleway_project_id = "abc-123"
    base_domain         = "example.org"
    gateway_domain      = "auth.example.org"
    infra_email         = "ops@example.org"
    # commented_key     = "ignored"
  }
}`
	cases := []struct{ key, want string }{
		{"scaleway_project_id", "abc-123"},
		{"base_domain", "example.org"},
		{"gateway_domain", "auth.example.org"},
		{"infra_email", "ops@example.org"},
		{"nonexistent", ""},
	}
	for _, tc := range cases {
		if got := GetString(tf, tc.key); got != tc.want {
			t.Errorf("GetString(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}

func TestFindApp(t *testing.T) {
	cases := []struct {
		name string
		want AppStatus
	}{
		{"outline", Enabled},
		{"vikunja", CommentedOut},
		{"espocrm", Disabled},
		{"grafana", Enabled}, // no explicit enabled line → treated as enabled
		{"nonexistent", Absent},
	}
	for _, tc := range cases {
		got, err := FindApp(sample, tc.name)
		if err != nil {
			t.Errorf("%s: err = %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestAddAppUncommentsBlock(t *testing.T) {
	out, err := AddApp(sample, "vikunja")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "# vikunja = {") {
		t.Errorf("vikunja block still commented")
	}
	if !strings.Contains(out, "vikunja = {") {
		t.Errorf("vikunja block missing")
	}
	status, _ := FindApp(out, "vikunja")
	if status != Enabled {
		t.Errorf("vikunja status after AddApp = %d, want Enabled (%d)", status, Enabled)
	}
}

func TestAddAppRefusesAlreadyEnabled(t *testing.T) {
	_, err := AddApp(sample, "outline")
	if err == nil {
		t.Fatal("expected error for already-enabled app")
	}
}

func TestAddAppFlipsDisabledBlock(t *testing.T) {
	out, err := AddApp(sample, "espocrm")
	if err != nil {
		t.Fatal(err)
	}
	status, _ := FindApp(out, "espocrm")
	if status != Enabled {
		t.Errorf("espocrm after AddApp = %d, want Enabled", status)
	}
}

func TestAddAppInsertsNewBlock(t *testing.T) {
	out, err := AddApp(sample, "n8n")
	if err != nil {
		t.Fatal(err)
	}
	status, _ := FindApp(out, "n8n")
	if status != Enabled {
		t.Errorf("n8n after AddApp = %d, want Enabled", status)
	}
	if !strings.Contains(out, "FIXME: set hostname") {
		t.Errorf("inserted block should have a FIXME hint:\n%s", out)
	}
}

func TestRemoveAppDisablesActiveBlock(t *testing.T) {
	out, err := RemoveApp(sample, "outline")
	if err != nil {
		t.Fatal(err)
	}
	status, _ := FindApp(out, "outline")
	if status != Disabled {
		t.Errorf("outline after RemoveApp = %d, want Disabled", status)
	}
}

func TestRemoveAppInsertsEnabledLineWhenMissing(t *testing.T) {
	out, err := RemoveApp(sample, "grafana")
	if err != nil {
		t.Fatal(err)
	}
	status, _ := FindApp(out, "grafana")
	if status != Disabled {
		t.Errorf("grafana after RemoveApp = %d, want Disabled", status)
	}
	if !strings.Contains(out, "enabled = false") {
		t.Errorf("expected inserted enabled line")
	}
}

func TestRemoveAppRefusesAbsent(t *testing.T) {
	_, err := RemoveApp(sample, "nonexistent")
	if err == nil {
		t.Fatal("expected error for absent app")
	}
}

func TestRemoveAppRefusesAlreadyDisabled(t *testing.T) {
	_, err := RemoveApp(sample, "espocrm")
	if err == nil {
		t.Fatal("expected error for already-disabled app")
	}
}

func TestRemoveAppRefusesCommented(t *testing.T) {
	_, err := RemoveApp(sample, "vikunja")
	if err == nil {
		t.Fatal("expected error for commented-out app")
	}
}
