// Package envvalues reads the consumer's committed, per-env, NON-secret
// values — the same files Terraform itself resolves, so the CLI never renders
// a file Terraform depends on and `terraform apply` works by hand with no CLI.
//
// Two layouts exist:
//
//   - four-layer (current): environments/<env>/env.yml, one file per env,
//     yamldecode'd by every layer root.
//   - legacy single-stack: environments/env-values.yml, one keyed file,
//     resolved by env.tf via basename(path.root).
//
// ForEnv / LoadResolved prefer the per-env file and fall back to the legacy
// keyed file, so preflight works against either layout.
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

// EnvYMLPath is environments/<env>/env.yml — the four-layer per-env file.
func EnvYMLPath(projectRoot, env string) string {
	return filepath.Join(projectRoot, "environments", env, "env.yml")
}

// ForEnv resolves one env's values: environments/<env>/env.yml when present
// (four-layer layout), else the env's block in the legacy env-values.yml.
func ForEnv(projectRoot, env string) (Slice, error) {
	path := EnvYMLPath(projectRoot, env)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Get(projectRoot, env)
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var s Slice
	if err := yaml.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(s) == 0 {
		return nil, fmt.Errorf("%s is empty — fill in the env's values", path)
	}
	return s, nil
}

// LoadResolved returns every env's resolved values: each environments/<env>/
// dir with an env.yml contributes that file; envs present only in the legacy
// env-values.yml contribute their block. Used for cross-env checks
// (CheckDistinctProjectIDs).
func LoadResolved(projectRoot string) (map[string]Slice, error) {
	all := map[string]Slice{}
	if legacy, err := Load(projectRoot); err == nil {
		for k, v := range legacy {
			all[k] = v
		}
	}
	entries, err := os.ReadDir(filepath.Join(projectRoot, "environments"))
	if err != nil {
		if len(all) > 0 {
			return all, nil
		}
		return nil, fmt.Errorf("read environments/: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		path := EnvYMLPath(projectRoot, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var s Slice
		if err := yaml.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		all[e.Name()] = s
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no env values found — expected environments/<env>/env.yml or environments/env-values.yml")
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
