// Package ansiblevars projects a subset of the env's terraform output
// JSON into the .ansible-vars.json file the playbooks read via -e@. The
// same projection up.sh and configure.sh do with jq.
package ansiblevars

import (
	"encoding/json"

	"github.com/sheyaln/sabokit-cli/internal/tfoutput"
)

// Keys lists the top-level TF output names that get pulled into
// .ansible-vars.json. The map flattens `.<name>.value → <name>` so the
// playbooks can reference them directly with `{{ <name> }}`.
var Keys = []string{
	"enabled_apps",
	"compute_hosts",
	"authentik_identity_domain",
	"identity_bootstrap",
	"traefik_acme_email",
	"split_dns_overrides",
	"monitoring_loki_push_url",
}

// Project reads a terraform output -json blob and returns the projected
// ansible-vars.json bytes (with a trailing newline). Missing keys are
// silently skipped — the original jq filter behaves the same when a TF
// output happens to be absent in a given env.
func Project(tfOutput []byte) ([]byte, error) {
	return ProjectKeys(tfOutput, Keys)
}

// ProjectKeys is Project but with an explicit key list; useful for tests
// or callers that need a different subset.
func ProjectKeys(tfOutput []byte, keys []string) ([]byte, error) {
	doc, err := tfoutput.Parse(tfOutput)
	if err != nil {
		return nil, err
	}
	out := make(map[string]json.RawMessage, len(keys))
	for _, k := range keys {
		if raw, ok := doc.Raw(k); ok {
			out[k] = raw
		}
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
