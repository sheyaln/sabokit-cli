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
