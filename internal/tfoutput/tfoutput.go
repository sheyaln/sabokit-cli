// Package tfoutput parses `terraform output -json` blobs.
//
// Each output is encoded as {"value":…,"sensitive":bool,"type":…}, and an
// env's outputs have heterogeneous value shapes — a string output (e.g.
// authentik_identity_domain) sits next to a map output (compute_hosts) next
// to an object (identity_bootstrap). So each value is held as raw JSON and
// decoded per-output into the Go type the caller expects.
//
// Decoding *every* value into one Go type — the recurring bug this package
// exists to prevent — fails the moment two outputs differ in shape (e.g.
// "cannot unmarshal string into Go struct field .value of type
// map[string]ComputeHost").
package tfoutput

import (
	"encoding/json"
	"fmt"
)

type entry struct {
	Value     json.RawMessage `json:"value"`
	Sensitive bool            `json:"sensitive"`
}

// Doc is a parsed terraform output document, keyed by output name.
type Doc map[string]entry

// Parse decodes a `terraform output -json` blob.
func Parse(raw []byte) (Doc, error) {
	var d Doc
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("parse terraform output: %w", err)
	}
	return d, nil
}

// Decode unmarshals the named output's value into dst (a pointer). The error
// names the output for both the missing and wrong-shape cases.
func (d Doc) Decode(name string, dst any) error {
	e, ok := d[name]
	if !ok {
		return fmt.Errorf("output %q not in terraform output", name)
	}
	if err := json.Unmarshal(e.Value, dst); err != nil {
		return fmt.Errorf("decode output %q: %w", name, err)
	}
	return nil
}

// String decodes the named output as a string.
func (d Doc) String(name string) (string, error) {
	var s string
	if err := d.Decode(name, &s); err != nil {
		return "", err
	}
	return s, nil
}

// Raw returns the named output's raw JSON value, or ok=false if absent.
func (d Doc) Raw(name string) (json.RawMessage, bool) {
	e, ok := d[name]
	return e.Value, ok
}
