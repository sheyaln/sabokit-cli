// Package inventory renders an ansible inventory.ini from the
// `compute_hosts` output of consumer-template's terraform stack. Replaces
// the inline python in up.sh.
package inventory

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ComputeHost mirrors the per-host shape consumer-template's TF emits.
type ComputeHost struct {
	PublicIP      string   `json:"public_ip"`
	AnsibleGroup  string   `json:"ansible_group"`
	AnsibleGroups []string `json:"ansible_groups"`
	AnsibleUser   string   `json:"ansible_user"`
}

// Render returns an INI-formatted inventory string. envName becomes the
// per-host suffix ("<short>-<env>"). Each host is added to its
// ansible_group and every entry in ansible_groups (deduped).
//
// Mirrors up.sh's python block: compute_hosts comes from the env's
// .tf-output.json under .compute_hosts.value, where each key is a "short"
// host name and the value is a ComputeHost.
func Render(envName string, hosts map[string]ComputeHost) string {
	byGroup := map[string][]string{}
	for short, h := range hosts {
		user := h.AnsibleUser
		if user == "" {
			user = "ubuntu"
		}
		line := fmt.Sprintf("%s-%s ansible_host=%s ansible_user=%s", short, envName, h.PublicIP, user)
		groups := dedupe(append([]string{h.AnsibleGroup}, h.AnsibleGroups...))
		for _, g := range groups {
			if g == "" {
				continue
			}
			byGroup[g] = append(byGroup[g], line)
		}
	}
	groupNames := make([]string, 0, len(byGroup))
	for g := range byGroup {
		groupNames = append(groupNames, g)
	}
	sort.Strings(groupNames)

	var b strings.Builder
	for _, g := range groupNames {
		fmt.Fprintf(&b, "[%s]\n", g)
		for _, line := range byGroup[g] {
			fmt.Fprintln(&b, line)
		}
		fmt.Fprintln(&b)
	}
	fmt.Fprintln(&b, "[all:vars]")
	fmt.Fprintln(&b, "ansible_python_interpreter=/usr/bin/python3")
	return b.String()
}

// FromTFOutput extracts compute_hosts from a `terraform output -json` blob
// and renders an inventory.
func FromTFOutput(envName string, tfOutput []byte) (string, error) {
	// Defer every output's value as raw JSON; only compute_hosts is a
	// map[string]ComputeHost. Sibling outputs (identity_domain, secret IDs,
	// infra_email, …) are strings — typing them all alike would fail here.
	var doc map[string]struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(tfOutput, &doc); err != nil {
		return "", fmt.Errorf("parse tf output: %w", err)
	}
	entry, ok := doc["compute_hosts"]
	if !ok {
		return "", fmt.Errorf("compute_hosts not in terraform output")
	}
	var hosts map[string]ComputeHost
	if err := json.Unmarshal(entry.Value, &hosts); err != nil {
		return "", fmt.Errorf("parse compute_hosts: %w", err)
	}
	if len(hosts) == 0 {
		return "", fmt.Errorf("compute_hosts is empty")
	}
	return Render(envName, hosts), nil
}

func dedupe(s []string) []string {
	seen := map[string]bool{}
	out := s[:0:0]
	for _, v := range s {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
