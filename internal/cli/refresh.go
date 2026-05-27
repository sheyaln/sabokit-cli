package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sheyaln/sabokit-cli/internal/ansiblevars"
	"github.com/sheyaln/sabokit-cli/internal/inventory"
	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/sheyaln/sabokit-cli/internal/tf"
)

// refreshEnvState pulls fresh terraform output for the env and rewrites
// the three derived files sabokit's other commands read:
//
//	.tf-output.json     raw `terraform output -json` snapshot
//	inventory.ini       ansible inventory built from compute_hosts
//	.ansible-vars.json  the playbook -e@ bundle (subset of TF outputs)
//
// Every command that talks to the env runs this first so a stale
// inventory or stale TF snapshot can't make ansible target the wrong
// host after a re-provision. Returns nil if no TF state exists yet AND
// the caller is OK with that (e.g. dry-run paths can swallow the error).
//
// envName is the directory name under environments/. envDir is the
// absolute host path. tfClient is the terraform docker client.
func refreshEnvState(envDir, envName string, tfClient *tf.Client) ([]byte, error) {
	tfOut, err := tfClient.Output(envDir)
	if err != nil {
		return nil, fmt.Errorf("read terraform state: %w (have you run 'sabokit up'?)", err)
	}
	trim := string(tfOut)
	if trim == "" || trim == "{}\n" || trim == "{}" {
		return nil, fmt.Errorf("terraform state is empty for env %s — run 'sabokit up' first", envName)
	}
	if err := os.WriteFile(filepath.Join(envDir, ".tf-output.json"), tfOut, 0o644); err != nil {
		return nil, err
	}
	ini, err := inventory.FromTFOutput(envName, tfOut)
	if err != nil {
		return nil, fmt.Errorf("regenerate inventory.ini: %w", err)
	}
	if err := os.WriteFile(filepath.Join(envDir, "inventory.ini"), []byte(ini), 0o644); err != nil {
		return nil, err
	}
	vars, err := ansiblevars.Project(tfOut)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(envDir, ".ansible-vars.json"), vars, 0o644); err != nil {
		return nil, err
	}
	return tfOut, nil
}

// refreshIfEnv refreshes when an env is configured; no-op otherwise. Used
// by deploy/down/status, which can technically run against the legacy flat
// layout (no envs at all).
func refreshIfEnv(p *project.Project, tfClient *tf.Client) error {
	envName := p.EnvName(globals.Env)
	if envName == "" {
		return nil
	}
	envDir, err := p.WorkspaceDir(globals.Env)
	if err != nil {
		return err
	}
	_, err = refreshEnvState(envDir, envName, tfClient)
	return err
}
