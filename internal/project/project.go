package project

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	configDir  = ".sabokit"
	configFile = "config.yml"
)

type Config struct {
	Project      string   `yaml:"project"`
	BaseDomain   string   `yaml:"base_domain"`
	DefaultEnv   string   `yaml:"default_env"`
	Scaleway     Scaleway `yaml:"scaleway"`
	SSH          SSH      `yaml:"ssh"`
	Inventory    string   `yaml:"inventory"`
	AppsManifest string   `yaml:"apps_manifest"`
}

type Scaleway struct {
	Region string `yaml:"region"`
	Zone   string `yaml:"zone"`
}

type SSH struct {
	User string `yaml:"user"`
	Key  string `yaml:"key"`
}

type Project struct {
	Root   string
	Config Config
}

func Load() (*Project, error) {
	root, err := findRoot()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, configDir, configFile))
	if err != nil {
		return nil, fmt.Errorf("read project config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse project config: %w", err)
	}
	if cfg.Inventory == "" {
		cfg.Inventory = "inventory.ini"
	}
	if cfg.AppsManifest == "" {
		cfg.AppsManifest = "apps-manifest.yaml"
	}
	return &Project{Root: root, Config: cfg}, nil
}

func findRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := cwd; ; {
		if fi, err := os.Stat(filepath.Join(dir, configDir, configFile)); err == nil && !fi.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no .sabokit/config.yml found in %s or any parent", cwd)
		}
		dir = parent
	}
}

func (p *Project) InventoryPath() string {
	return filepath.Join(p.Root, p.Config.Inventory)
}

// Layers are the four-layer composition roots under environments/<env>/, in
// apply order. Teardown runs them in reverse.
var Layers = []string{"infra", "identity", "operations", "application"}

// IsFourLayer reports whether the env follows the four-layer layout:
// environments/<env>/env.yml plus per-layer roots (infra/stack.tf as the
// sentinel). Legacy envs carry a single main.tf instead.
func (p *Project) IsFourLayer(envOverride string) bool {
	env := p.EnvName(envOverride)
	if env == "" {
		return false
	}
	dir := filepath.Join(p.Root, "environments", env)
	if _, err := os.Stat(filepath.Join(dir, "env.yml")); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "infra", "stack.tf"))
	return err == nil
}

// Envs returns the environment names under environments/ — directories that
// contain either a four-layer infra/stack.tf or a legacy main.tf, excluding
// _template.
func (p *Project) Envs() []string {
	entries, err := os.ReadDir(filepath.Join(p.Root, "environments"))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		dir := filepath.Join(p.Root, "environments", e.Name())
		if _, err := os.Stat(filepath.Join(dir, "infra", "stack.tf")); err == nil {
			out = append(out, e.Name())
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "main.tf")); err == nil {
			out = append(out, e.Name())
		}
	}
	return out
}

func (p *Project) AppsManifestPath() string {
	return filepath.Join(p.Root, p.Config.AppsManifest)
}

// EnvName returns the effective env name (override > config default). Empty
// string means "no env" — sabokit operates against the project root in that
// mode (legacy / flat-layout projects).
func (p *Project) EnvName(override string) string {
	if override != "" {
		return override
	}
	return p.Config.DefaultEnv
}

// WorkspaceDir returns the host path that should be mounted as /workspace
// inside the runner container. When an env is set, that's
// <root>/environments/<env>/; otherwise the project root.
func (p *Project) WorkspaceDir(envOverride string) (string, error) {
	env := p.EnvName(envOverride)
	if env == "" {
		return p.Root, nil
	}
	dir := filepath.Join(p.Root, "environments", env)
	fi, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("environment %q not found: %s", env, dir)
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("environment path %s is not a directory", dir)
	}
	return dir, nil
}

// EnabledAppsPath returns the host path to .enabled_apps.json for the
// current env (written by scripts/refresh.sh and the layer scripts), or ""
// when no env is configured.
func (p *Project) EnabledAppsPath(envOverride string) string {
	dir, err := p.WorkspaceDir(envOverride)
	if err != nil || p.EnvName(envOverride) == "" {
		return ""
	}
	return filepath.Join(dir, ".enabled_apps.json")
}

type appsManifest struct {
	SchemaVersion int           `yaml:"schema_version"`
	Apps          []manifestApp `yaml:"apps"`
}

type manifestApp struct {
	ID               string `yaml:"id"`
	DisplayName      string `yaml:"display_name"`
	Category         string `yaml:"category"`
	DescriptionShort string `yaml:"description_short"`
}

// CatalogApp is one entry in the upstream apps-manifest.yaml. The manifest
// is a producer-curated catalog of every app sabokit knows how to deploy.
// Per-env enabled state and host assignment do NOT live here — those come
// from the env's .ansible-vars.json (written by up.sh).
type CatalogApp struct {
	ID               string
	DisplayName      string
	Category         string
	DescriptionShort string
}

func (p *Project) Catalog() ([]CatalogApp, error) {
	data, err := os.ReadFile(p.AppsManifestPath())
	if err != nil {
		return nil, fmt.Errorf("read apps manifest: %w", err)
	}
	var m appsManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse apps manifest: %w", err)
	}
	out := make([]CatalogApp, 0, len(m.Apps))
	for _, a := range m.Apps {
		out = append(out, CatalogApp{
			ID:               a.ID,
			DisplayName:      a.DisplayName,
			Category:         a.Category,
			DescriptionShort: a.DescriptionShort,
		})
	}
	return out, nil
}

// EnvApp is the env-resolved view of an app: enabled state plus the URL TF
// computed for it (when enabled). Sourced from .enabled_apps.json's
// enabled_apps map.
type EnvApp struct {
	ID      string
	Enabled bool
	URL     string
}

func (p *Project) EnvApps(envOverride string) ([]EnvApp, error) {
	catalog, err := p.Catalog()
	if err != nil {
		return nil, err
	}
	enabled, err := p.loadEnabledAppsMap(envOverride)
	if err != nil {
		return nil, err
	}
	out := make([]EnvApp, 0, len(catalog))
	for _, a := range catalog {
		e, ok := enabled[a.ID]
		isEnabled := ok && e.URL != ""
		out = append(out, EnvApp{ID: a.ID, Enabled: isEnabled, URL: e.URL})
	}
	return out, nil
}

// InventoryHosts parses the env's inventory.ini and returns the host names
// in the named group ("apps", "identity", etc.). Group sections like
// "[apps:vars]" or "[all:vars]" are ignored (they declare variables, not
// hosts).
func (p *Project) InventoryHosts(envOverride, group string) ([]string, error) {
	dir, err := p.WorkspaceDir(envOverride)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, p.Config.Inventory)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open inventory %s: %w", path, err)
	}
	defer f.Close()

	var hosts []string
	currentGroup := ""
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			header := line[1 : len(line)-1]
			if strings.Contains(header, ":") {
				currentGroup = ""
				continue
			}
			currentGroup = header
			continue
		}
		if currentGroup != group {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		hosts = append(hosts, fields[0])
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return hosts, nil
}

type enabledAppsFile struct {
	EnabledApps map[string]ansibleEnabledApp `json:"enabled_apps"`
}

type ansibleEnabledApp struct {
	URL string `json:"url"`
}

func (p *Project) loadEnabledAppsMap(envOverride string) (map[string]ansibleEnabledApp, error) {
	path := p.EnabledAppsPath(envOverride)
	if path == "" {
		return nil, fmt.Errorf("no env set (pass --env or set default_env in .sabokit/config.yml)")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w (run 'sabokit refresh' or scripts/refresh.sh to generate)", path, err)
	}
	var v enabledAppsFile
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return v.EnabledApps, nil
}
