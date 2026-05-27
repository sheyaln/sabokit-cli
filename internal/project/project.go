package project

import (
	"fmt"
	"os"
	"path/filepath"

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

// AnsibleVarsPath returns the host path to .ansible-vars.json for the
// current env, or "" when no env is configured.
func (p *Project) AnsibleVarsPath(envOverride string) string {
	dir, err := p.WorkspaceDir(envOverride)
	if err != nil || p.EnvName(envOverride) == "" {
		return ""
	}
	return filepath.Join(dir, ".ansible-vars.json")
}

// TFOutputPath returns the host path to .tf-output.json for the current env.
func (p *Project) TFOutputPath(envOverride string) string {
	dir, err := p.WorkspaceDir(envOverride)
	if err != nil || p.EnvName(envOverride) == "" {
		return ""
	}
	return filepath.Join(dir, ".tf-output.json")
}

type appsManifest struct {
	Apps map[string]appEntry `yaml:"apps"`
}

type appEntry struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
}

type App struct {
	Name    string
	Enabled bool
	Host    string
}

func (p *Project) AllApps() ([]App, error) {
	data, err := os.ReadFile(p.AppsManifestPath())
	if err != nil {
		return nil, fmt.Errorf("read apps manifest: %w", err)
	}
	var m appsManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse apps manifest: %w", err)
	}
	out := make([]App, 0, len(m.Apps))
	for name, e := range m.Apps {
		out = append(out, App{Name: name, Enabled: e.Enabled, Host: e.Host})
	}
	return out, nil
}

func (p *Project) HostsForApp(name string) ([]string, error) {
	data, err := os.ReadFile(p.AppsManifestPath())
	if err != nil {
		return nil, fmt.Errorf("read apps manifest: %w", err)
	}
	var m appsManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse apps manifest: %w", err)
	}
	e, ok := m.Apps[name]
	if !ok {
		return nil, fmt.Errorf("app %q not in manifest", name)
	}
	if e.Host == "" {
		return nil, nil
	}
	return []string{e.Host}, nil
}

func (p *Project) EnabledApps() ([]string, error) {
	data, err := os.ReadFile(p.AppsManifestPath())
	if err != nil {
		return nil, fmt.Errorf("read apps manifest: %w", err)
	}
	var m appsManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse apps manifest: %w", err)
	}
	out := make([]string, 0, len(m.Apps))
	for name, e := range m.Apps {
		if e.Enabled {
			out = append(out, name)
		}
	}
	return out, nil
}
