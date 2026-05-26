package scw

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

const sampleSecretList = `[
	{"id":"9d6e3895-18ce-4ad9-a059-dd1d24d592f7","name":"authentik-admin-password","tags":["authentik"],"version_count":1,"updated_at":"2025-04-01T10:00:00Z","status":"ready","description":"Authentik admin password","type":"opaque"},
	{"id":"5e6e4c2e-7dff-480a-b680-e3a07f9279c0","name":"authentik-app-espocrm","tags":["authentik","espocrm","oidc"],"version_count":1,"updated_at":"2025-04-02T10:00:00Z","status":"ready","type":"key_value"},
	{"id":"8ca9a6b6-4f11-48b4-97d1-c9013d317939","name":"backrest-authentik-restic-password","tags":["backrest"],"version_count":1,"updated_at":"2025-04-03T10:00:00Z","status":"ready","type":"opaque"}
]`

func TestParseIndex(t *testing.T) {
	c := &Client{image: "test:v1", platform: ""}
	idx, err := c.parseIndex([]byte(sampleSecretList))
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.all) != 3 {
		t.Fatalf("got %d secrets, want 3", len(idx.all))
	}
	want := map[string]string{
		"authentik-admin-password":           "9d6e3895-18ce-4ad9-a059-dd1d24d592f7",
		"authentik-app-espocrm":              "5e6e4c2e-7dff-480a-b680-e3a07f9279c0",
		"backrest-authentik-restic-password": "8ca9a6b6-4f11-48b4-97d1-c9013d317939",
	}
	for name, wantID := range want {
		s, ok := idx.byName[name]
		if !ok {
			t.Errorf("missing %q in index", name)
			continue
		}
		if s.ID != wantID {
			t.Errorf("%q: id = %q, want %q", name, s.ID, wantID)
		}
	}
}

func TestResolveUsesCache(t *testing.T) {
	c := &Client{image: "test:v1", platform: ""}
	if _, err := c.parseIndex([]byte(sampleSecretList)); err != nil {
		t.Fatal(err)
	}
	s, err := c.Resolve("authentik-admin-password")
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != "9d6e3895-18ce-4ad9-a059-dd1d24d592f7" {
		t.Errorf("got id %q", s.ID)
	}
}

func TestResolveMissing(t *testing.T) {
	c := &Client{image: "test:v1", platform: ""}
	if _, err := c.parseIndex([]byte(sampleSecretList)); err != nil {
		t.Fatal(err)
	}
	_, err := c.Resolve("nonexistent-secret")
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error %q should mention 'not found'", err.Error())
	}
}

func TestAccessVersionDecodesBase64(t *testing.T) {
	original := []byte("super-secret-value")
	encoded := base64.StdEncoding.EncodeToString(original)
	resp := versionAccess{Data: encoded}
	raw, _ := json.Marshal(resp)

	var v versionAccess
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(v.Data)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != string(original) {
		t.Errorf("decoded = %q, want %q", decoded, original)
	}
}

func TestParseRevision(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"latest", "latest", false},
		{"1", "1", false},
		{"42", "42", false},
		{"abc", "", true},
		{"", "", true},
		{"-1", "-1", false},
	}
	for _, tc := range cases {
		got, err := ParseRevision(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("%q: err = %v, wantErr = %v", tc.in, err, tc.wantErr)
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.in, got, tc.want)
		}
	}
}
