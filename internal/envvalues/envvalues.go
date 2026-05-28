// Package envvalues reads the consumer's environments/env-values.yml — the
// committed, per-env, keyed source of NON-secret values that Terraform itself
// resolves via env.tf (yamldecode + basename(path.root)). The CLI reads the
// same file for preflight checks; it never renders a file Terraform depends on,
// so `terraform apply` works by hand with no CLI.
package envvalues

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Slice is one env's block. Values may be scalars or nested maps
// (compute_instance_types, compute_disk_sizes), so it stays loosely typed.
type Slice map[string]any

// RequiredKeys have no default in env.tf — a deploy can't proceed without them.
var RequiredKeys = []string{"scaleway_project_id", "base_domain", "identity_domain", "infra_email"}

// Path is environments/env-values.yml under the project root.
func Path(projectRoot string) string {
	return filepath.Join(projectRoot, "environments", "env-values.yml")
}

// Load parses environments/env-values.yml into a map keyed by env name.
func Load(projectRoot string) (map[string]Slice, error) {
	path := Path(projectRoot)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var all map[string]Slice
	if err := yaml.Unmarshal(raw, &all); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("%s has no env blocks — add an <env>: key", path)
	}
	return all, nil
}

// Get returns the slice for env, with a clear error listing the available envs
// when the key is absent (env = the directory name terraform selects by).
func Get(projectRoot, env string) (Slice, error) {
	all, err := Load(projectRoot)
	if err != nil {
		return nil, err
	}
	s, ok := all[env]
	if !ok {
		return nil, fmt.Errorf("env %q has no block in %s (available: %s)", env, Path(projectRoot), strings.Join(Names(all), ", "))
	}
	return s, nil
}

// String returns key as a string, or "" if absent or not a string scalar.
func (s Slice) String(key string) string {
	if v, ok := s[key]; ok {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return ""
}

// Require errors if any of keys is missing or empty in the slice.
func (s Slice) Require(keys ...string) error {
	var missing []string
	for _, k := range keys {
		if s.String(k) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing/empty required key(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

// CheckDistinctProjectIDs errors if two envs share a scaleway_project_id — the
// exact copy/paste hazard (staging's project landing in prod).
func CheckDistinctProjectIDs(all map[string]Slice) error {
	seen := map[string]string{} // project_id -> env
	for _, env := range Names(all) {
		pid := all[env].String("scaleway_project_id")
		if pid == "" {
			continue
		}
		if other, dup := seen[pid]; dup {
			return fmt.Errorf("envs %q and %q share scaleway_project_id %q — each env needs a distinct project", other, env, pid)
		}
		seen[pid] = env
	}
	return nil
}

// Names returns env keys in deterministic (sorted) order.
func Names(all map[string]Slice) []string {
	names := make([]string, 0, len(all))
	for k := range all {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
