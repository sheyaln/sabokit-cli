package ansiblevars

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProject(t *testing.T) {
	raw := `{
		"enabled_apps":             {"value": {"outline": {"url": "https://wiki"}}},
		"compute_hosts":            {"value": {"apps": {"public_ip": "1.2.3.4"}}},
		"authentik_identity_domain": {"value": "auth.example.org"},
		"infra_email":              {"value": "ops@example.org"},
		"unrelated":                {"value": "should be excluded"}
	}`
	got, err := Project([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc["enabled_apps"]; !ok {
		t.Errorf("enabled_apps missing")
	}
	if _, ok := doc["authentik_identity_domain"]; !ok {
		t.Errorf("authentik_identity_domain missing")
	}
	if _, ok := doc["unrelated"]; ok {
		t.Errorf("unrelated should be filtered out")
	}
	if !strings.HasSuffix(string(got), "\n") {
		t.Errorf("expected trailing newline")
	}
}

func TestProjectMissingKeysIsOK(t *testing.T) {
	raw := `{"enabled_apps": {"value": {}}}`
	got, err := Project([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc) != 1 {
		t.Errorf("expected 1 key, got %d", len(doc))
	}
}
