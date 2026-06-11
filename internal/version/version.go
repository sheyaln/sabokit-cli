// Package version holds the CLI's own version and the range of sabokit
// blueprint versions it is compatible with. The CLI does not choose a
// blueprint version — the environment does, via the ?ref= its terraform
// pins. The CLI reads that pin, runs the matching runner image, and refuses
// to act when the pinned version falls outside the range declared here.
//
// CLI and SupportedBlueprintMax are injected via ldflag at release-build time
// (see .github/workflows/release.yml); the defaults below are what a
// locally-built / dev binary reports.
package version

import (
	"fmt"
	"strconv"
	"strings"
)

var (
	// CLI is the sabokit-cli semver, injected via ldflag at build time.
	CLI = "0.2.0-dev"

	// SupportedBlueprintMin is the oldest blueprint major.minor line this CLI
	// can drive. It is a source constant — bump it when dropping support for
	// an old line. 0.2 is the floor: the CLI drives the four-layer roots via
	// the consumer-template layer scripts, which v0.1's single-stack layout
	// doesn't have.
	SupportedBlueprintMin = "0.2"

	// SupportedBlueprintMax is the newest blueprint major.minor line this CLI
	// was built for. Injected at build = the CLI tag's own major.minor.
	SupportedBlueprintMax = "0.2"
)

// MajorMinor extracts the "X.Y" line from a version or ref string. It accepts
// a leading "v", a prerelease suffix ("-beta1"), and an optional patch
// component: "v0.1.0-beta1", "0.1.0", and "0.1" all yield "0.1".
func MajorMinor(s string) (string, error) {
	maj, min, err := parseLine(s)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d.%d", maj, min), nil
}

// Supports reports whether the given blueprint ref (vX.Y.Z[-pre]) falls within
// this CLI's supported [min, max] major.minor range, inclusive.
func Supports(ref string) (bool, error) {
	maj, min, err := parseLine(ref)
	if err != nil {
		return false, err
	}
	loMaj, loMin, err := parseLine(SupportedBlueprintMin)
	if err != nil {
		return false, fmt.Errorf("SupportedBlueprintMin %q: %w", SupportedBlueprintMin, err)
	}
	hiMaj, hiMin, err := parseLine(SupportedBlueprintMax)
	if err != nil {
		return false, fmt.Errorf("SupportedBlueprintMax %q: %w", SupportedBlueprintMax, err)
	}
	return !less(maj, min, loMaj, loMin) && !less(hiMaj, hiMin, maj, min), nil
}

// SupportedRange is the human-readable "min–max" (or "X.Y" when min == max)
// for error messages and `sabokit version`.
func SupportedRange() string {
	if SupportedBlueprintMin == SupportedBlueprintMax {
		return "v" + SupportedBlueprintMin
	}
	return "v" + SupportedBlueprintMin + "–v" + SupportedBlueprintMax
}

// less reports whether (aMaj,aMin) sorts before (bMaj,bMin).
func less(aMaj, aMin, bMaj, bMin int) bool {
	if aMaj != bMaj {
		return aMaj < bMaj
	}
	return aMin < bMin
}

// parseLine returns the major and minor ints from a version/ref/line string.
func parseLine(s string) (int, int, error) {
	v := strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexByte(v, '-'); i >= 0 { // drop prerelease suffix
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("not a major.minor version: %q", s)
	}
	maj, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("bad major in %q: %w", s, err)
	}
	min, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("bad minor in %q: %w", s, err)
	}
	return maj, min, nil
}
