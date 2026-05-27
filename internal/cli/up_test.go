package cli

import (
	"strings"
	"testing"
)

func TestConfirmPlanProdRequiresExplicitYes(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"yes\n", true},
		{"Y\n", true},
		{"\n", false},   // enter on prod aborts
		{"n\n", false},
		{"no\n", false},
		{"maybe\n", false},
	}
	for _, tc := range cases {
		got, err := confirmPlan("prod", strings.NewReader(tc.input))
		if err != nil {
			t.Errorf("prod %q: err = %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("prod %q: got %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestConfirmPlanNonProdDefaultsYes(t *testing.T) {
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
		got, err := confirmPlan("staging", strings.NewReader(tc.input))
		if err != nil {
			t.Errorf("staging %q: err = %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("staging %q: got %v, want %v", tc.input, got, tc.want)
		}
	}
}
