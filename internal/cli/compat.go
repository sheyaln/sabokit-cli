package cli

import (
	"fmt"

	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/sheyaln/sabokit-cli/internal/version"
)

// requireCompatibleBlueprint fails fast when the sabokit version the current
// environment pins falls outside the range this CLI supports. It reads the
// pin from the env's terraform (the single source of truth) — the CLI never
// chooses the version, it conforms to and verifies it. Bypass with
// --skip-version-check.
func requireCompatibleBlueprint(p *project.Project) error {
	if globals.SkipVersionCheck {
		return nil
	}
	// Flat-layout / no-env projects can't pin per-env; nothing to check.
	if p.EnvName(globals.Env) == "" {
		return nil
	}
	ref, err := p.BlueprintVersion(globals.Env)
	if err != nil {
		return fmt.Errorf("could not determine the sabokit version this environment pins: %w", err)
	}
	ok, err := version.Supports(ref)
	if err != nil {
		return fmt.Errorf("parse pinned sabokit version %q: %w", ref, err)
	}
	if !ok {
		return fmt.Errorf(`environment pins sabokit %s, but this sabokit CLI (%s) supports blueprints %s.
  → upgrade the CLI:  curl -fsSL https://raw.githubusercontent.com/sheyaln/sabokit-cli/master/install.sh | bash
  → or re-pin the env to a supported sabokit version (the ?ref= in each environments/<env>/<layer>/stack.tf)
  override (unsafe — mismatched terraform/ansible can corrupt state): --skip-version-check`,
			ref, version.CLI, version.SupportedRange())
	}
	return nil
}
