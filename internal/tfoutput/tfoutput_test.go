package tfoutput

import "testing"

// The core regression: a string-shaped output sitting next to a map-shaped
// output must not derail decoding the map. The old code typed every value
// alike and blew up here.
const mixed = `{
	"authentik_identity_domain": { "value": "https://sso.example.org", "sensitive": false },
	"enabled_apps":              { "value": ["a","b"], "sensitive": true },
	"compute_hosts": {
		"value": { "tools": { "public_ip": "1.2.3.4", "ansible_group": "tools" } },
		"sensitive": false
	}
}`

func TestDecodeMapBesideString(t *testing.T) {
	doc, err := Parse([]byte(mixed))
	if err != nil {
		t.Fatal(err)
	}

	var hosts map[string]struct {
		PublicIP     string `json:"public_ip"`
		AnsibleGroup string `json:"ansible_group"`
	}
	if err := doc.Decode("compute_hosts", &hosts); err != nil {
		t.Fatalf("decode compute_hosts: %v", err)
	}
	if hosts["tools"].PublicIP != "1.2.3.4" {
		t.Errorf("tools public_ip = %q", hosts["tools"].PublicIP)
	}

	dom, err := doc.String("authentik_identity_domain")
	if err != nil || dom != "https://sso.example.org" {
		t.Errorf("String = %q, err %v", dom, err)
	}
}

func TestDecodeMissing(t *testing.T) {
	doc, _ := Parse([]byte(`{}`))
	if err := doc.Decode("compute_hosts", &map[string]any{}); err == nil {
		t.Error("expected error for missing output")
	}
}

func TestRaw(t *testing.T) {
	doc, _ := Parse([]byte(mixed))
	if raw, ok := doc.Raw("enabled_apps"); !ok || string(raw) != `["a","b"]` {
		t.Errorf("Raw = %s, ok %v", raw, ok)
	}
	if _, ok := doc.Raw("nope"); ok {
		t.Error("expected ok=false for absent output")
	}
}
