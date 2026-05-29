package inventory

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	hosts := map[string]ComputeHost{
		"apps": {
			PublicIP:     "51.1.1.1",
			AnsibleGroup: "apps",
			AnsibleUser:  "root",
		},
		"identity": {
			PublicIP:      "51.2.2.2",
			AnsibleGroup:  "identity",
			AnsibleGroups: []string{"apps"},
			AnsibleUser:   "root",
		},
	}
	got := Render("prod", hosts)
	if !strings.Contains(got, "[apps]") {
		t.Errorf("missing [apps] section: %s", got)
	}
	if !strings.Contains(got, "[identity]") {
		t.Errorf("missing [identity] section: %s", got)
	}
	if !strings.Contains(got, "apps-prod ansible_host=51.1.1.1") {
		t.Errorf("missing apps-prod line: %s", got)
	}
	if !strings.Contains(got, "identity-prod ansible_host=51.2.2.2") {
		t.Errorf("missing identity-prod line: %s", got)
	}
	// identity host has ansible_groups including "apps" so it should be
	// listed under [apps] AS WELL.
	appsSection := sectionOf(got, "apps")
	if !strings.Contains(appsSection, "identity-prod") {
		t.Errorf("identity-prod should appear in [apps] via ansible_groups: %q", appsSection)
	}
	if !strings.Contains(got, "[all:vars]") {
		t.Errorf("missing [all:vars]")
	}
}

func TestRenderDefaultUser(t *testing.T) {
	hosts := map[string]ComputeHost{
		"apps": {PublicIP: "51.1.1.1", AnsibleGroup: "apps"},
	}
	got := Render("dev", hosts)
	if !strings.Contains(got, "ansible_user=ubuntu") {
		t.Errorf("expected default user=ubuntu: %s", got)
	}
}

func TestFromTFOutput(t *testing.T) {
	// Sibling string-valued outputs must not derail parsing — only
	// compute_hosts is typed as a host map.
	raw := `{
		"authentik_identity_domain": { "value": "https://sso.example.org" },
		"infra_email": { "value": "ops@example.org" },
		"compute_hosts": {
			"value": {
				"apps": {
					"public_ip": "51.1.1.1",
					"ansible_group": "apps",
					"ansible_user": "root"
				}
			}
		}
	}`
	got, err := FromTFOutput("staging", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "apps-staging ansible_host=51.1.1.1") {
		t.Errorf("rendered = %s", got)
	}
}

func TestFromTFOutputMissingHosts(t *testing.T) {
	_, err := FromTFOutput("dev", []byte(`{}`))
	if err == nil {
		t.Error("expected error for empty TF output")
	}
}

func sectionOf(out, name string) string {
	header := "[" + name + "]"
	idx := strings.Index(out, header)
	if idx < 0 {
		return ""
	}
	rest := out[idx+len(header):]
	end := strings.Index(rest, "\n[")
	if end < 0 {
		return rest
	}
	return rest[:end]
}
