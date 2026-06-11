package cli

import (
	"strings"
	"testing"

	"github.com/sheyaln/sabokit-cli/internal/project"
)

func TestConfirmUpProdRequiresExplicitYes(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"yes\n", true},
		{"Y\n", true},
		{"\n", false}, // enter on prod aborts
		{"n\n", false},
		{"no\n", false},
		{"maybe\n", false},
	}
	for _, tc := range cases {
		got, err := confirmUp("prod", "proj-id", project.Layers, strings.NewReader(tc.input))
		if err != nil {
			t.Errorf("prod %q: err = %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("prod %q: got %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestConfirmUpNonProdDefaultsYes(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"\n", true}, // enter on non-prod proceeds
		{"y\n", true},
		{"n\n", false},
		{"no\n", false},
	}
	for _, tc := range cases {
		got, err := confirmUp("staging", "proj-id", project.Layers, strings.NewReader(tc.input))
		if err != nil {
			t.Errorf("staging %q: err = %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("staging %q: got %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestResolveLayers(t *testing.T) {
	all, err := resolveLayers(nil)
	if err != nil || len(all) != 4 {
		t.Fatalf("default layers = %v, %v", all, err)
	}
	subset, err := resolveLayers([]string{"application", "operations"})
	if err != nil {
		t.Fatal(err)
	}
	// normalised to dependency order
	if len(subset) != 2 || subset[0] != "operations" || subset[1] != "application" {
		t.Errorf("subset = %v, want [operations application]", subset)
	}
	if _, err := resolveLayers([]string{"base"}); err == nil {
		t.Error("expected error for unknown layer name")
	}
}
