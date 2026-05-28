package version

import "testing"

func TestMajorMinor(t *testing.T) {
	cases := map[string]string{
		"v0.1.0":       "0.1",
		"0.1.0":        "0.1",
		"v0.1":         "0.1",
		"v0.1.0-beta1": "0.1",
		"v12.7.3":      "12.7",
	}
	for in, want := range cases {
		got, err := MajorMinor(in)
		if err != nil {
			t.Fatalf("MajorMinor(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("MajorMinor(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := MajorMinor("nope"); err == nil {
		t.Error("MajorMinor(nope): want error")
	}
}

func TestSupports(t *testing.T) {
	defer func(lo, hi string) { SupportedBlueprintMin, SupportedBlueprintMax = lo, hi }(SupportedBlueprintMin, SupportedBlueprintMax)
	SupportedBlueprintMin, SupportedBlueprintMax = "0.1", "0.3"
	in := map[string]bool{
		"v0.1.0":       true,
		"v0.1.9-beta2": true,
		"v0.2.5":       true,
		"v0.3.0":       true,
		"v0.0.9":       false,
		"v0.4.0":       false,
		"v1.0.0":       false,
	}
	for ref, want := range in {
		got, err := Supports(ref)
		if err != nil {
			t.Fatalf("Supports(%q): %v", ref, err)
		}
		if got != want {
			t.Errorf("Supports(%q) = %v, want %v", ref, got, want)
		}
	}
}
