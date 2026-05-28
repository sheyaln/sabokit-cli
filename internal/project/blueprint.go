package project

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	stackModuleRe = regexp.MustCompile(`(?m)^\s*module\s+"stack"\s*\{`)
	sourceRe      = regexp.MustCompile(`(?m)^\s*source\s*=\s*"([^"]+)"`)
	refRe         = regexp.MustCompile(`\?ref=([A-Za-z0-9._-]+)`)
)

// BlueprintVersion resolves the sabokit version the given environment pins, by
// reading environments/<env>/main.tf's `module "stack"` source:
//
//   - a remote source (git::...?ref=vX.Y.Z) yields that ref directly;
//   - a local source (../../modules/stack) is the canonical vendored pattern —
//     the version is the unique ?ref= across that module's *.tf files.
//
// Returns the ref string, eg. "v0.1.0".
func (p *Project) BlueprintVersion(envOverride string) (string, error) {
	dir, err := p.WorkspaceDir(envOverride)
	if err != nil {
		return "", err
	}
	mainTF := filepath.Join(dir, "main.tf")
	src, err := stackSource(mainTF)
	if err != nil {
		return "", err
	}
	if m := refRe.FindStringSubmatch(src); m != nil {
		return m[1], nil
	}
	// Vendored: resolve the module dir relative to the env dir and read its
	// inner sub-module pins.
	return uniqueRef(filepath.Clean(filepath.Join(dir, src)))
}

// stackSource extracts the source string of the `module "stack"` block.
func stackSource(mainTFPath string) (string, error) {
	data, err := os.ReadFile(mainTFPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", mainTFPath, err)
	}
	loc := stackModuleRe.FindIndex(data)
	if loc == nil {
		return "", fmt.Errorf(`no module "stack" block in %s`, mainTFPath)
	}
	m := sourceRe.FindSubmatch(data[loc[1]:])
	if m == nil {
		return "", fmt.Errorf(`module "stack" in %s has no source`, mainTFPath)
	}
	return string(m[1]), nil
}

// uniqueRef collects the distinct ?ref= pins across dir/*.tf and returns the
// single value, erroring on none or on a mix (a half-applied version bump).
func uniqueRef(dir string) (string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.tf"))
	if err != nil {
		return "", err
	}
	set := map[string]struct{}{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		for _, m := range refRe.FindAllStringSubmatch(string(data), -1) {
			set[m[1]] = struct{}{}
		}
	}
	switch len(set) {
	case 0:
		return "", fmt.Errorf("no ?ref= pins found under %s (is the stack module vendored?)", dir)
	case 1:
		for r := range set {
			return r, nil
		}
	}
	refs := make([]string, 0, len(set))
	for r := range set {
		refs = append(refs, r)
	}
	sort.Strings(refs)
	return "", fmt.Errorf("ambiguous sabokit version under %s: multiple refs pinned (%s) — unify them with a version bump", dir, strings.Join(refs, ", "))
}
