package scw

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/sheyaln/sabokit-cli/internal/docker"
)

var envPassthru = []string{
	"SCW_ACCESS_KEY",
	"SCW_SECRET_KEY",
	"SCW_DEFAULT_PROJECT_ID",
	"SCW_DEFAULT_ORGANIZATION_ID",
	"SCW_DEFAULT_REGION",
	"SCW_DEFAULT_ZONE",
}

type Client struct {
	image     string
	platform  string
	cachedIdx *secretIndex
}

func New(image, platform string) *Client {
	return &Client{image: image, platform: platform}
}

type Secret struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Tags         []string `json:"tags"`
	VersionCount int      `json:"version_count"`
	UpdatedAt    string   `json:"updated_at"`
	CreatedAt    string   `json:"created_at"`
	Status       string   `json:"status"`
	Description  string   `json:"description"`
	Type         string   `json:"type"`
}

type Version struct {
	Revision  int    `json:"revision"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type secretIndex struct {
	all    []Secret
	byName map[string]Secret
}

func (c *Client) invocation(args ...string) docker.Invocation {
	return docker.Invocation{
		Image:       c.image,
		Platform:    c.platform,
		Entrypoint:  "scw",
		Cmd:         args,
		EnvPassthru: envPassthru,
		TTY:         false,
	}
}

func (c *Client) run(args []string) ([]byte, error) {
	inv := c.invocation(args...)
	var stdout, stderr bytes.Buffer
	cmd := inv.Command()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("scw %v: %w\n%s", args, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func (c *Client) index() (*secretIndex, error) {
	if c.cachedIdx != nil {
		return c.cachedIdx, nil
	}
	raw, err := c.run([]string{"secret", "secret", "list", "-o", "json"})
	if err != nil {
		return nil, err
	}
	return c.parseIndex(raw)
}

func (c *Client) parseIndex(raw []byte) (*secretIndex, error) {
	var arr []Secret
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("parse scw secret list: %w", err)
	}
	idx := &secretIndex{
		all:    arr,
		byName: make(map[string]Secret, len(arr)),
	}
	for _, s := range arr {
		idx.byName[s.Name] = s
	}
	c.cachedIdx = idx
	return idx, nil
}

func (c *Client) ListSecrets() ([]Secret, error) {
	idx, err := c.index()
	if err != nil {
		return nil, err
	}
	return idx.all, nil
}

func (c *Client) Resolve(name string) (Secret, error) {
	idx, err := c.index()
	if err != nil {
		return Secret{}, err
	}
	s, ok := idx.byName[name]
	if !ok {
		return Secret{}, fmt.Errorf("secret %q not found (use 'sabokit secrets list' to see available names)", name)
	}
	return s, nil
}

func (c *Client) ListVersions(secretID string) ([]Version, error) {
	raw, err := c.run([]string{"secret", "version", "list", "secret-id=" + secretID, "-o", "json"})
	if err != nil {
		return nil, err
	}
	var vs []Version
	if err := json.Unmarshal(raw, &vs); err != nil {
		return nil, fmt.Errorf("parse scw version list: %w", err)
	}
	return vs, nil
}

type versionAccess struct {
	Data string `json:"data"`
}

func (c *Client) AccessVersion(secretID, revision string) ([]byte, error) {
	raw, err := c.run([]string{"secret", "version", "access",
		"secret-id=" + secretID,
		"revision=" + revision,
		"-o", "json",
	})
	if err != nil {
		return nil, err
	}
	var v versionAccess
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("parse scw version access: %w", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(v.Data)
	if err != nil {
		return nil, fmt.Errorf("decode base64 data: %w", err)
	}
	return decoded, nil
}

func (c *Client) CreateSecret(name string, tags []string, description string) (Secret, error) {
	args := []string{"secret", "secret", "create", "name=" + name}
	for i, t := range tags {
		args = append(args, fmt.Sprintf("tags.%d=%s", i, t))
	}
	if description != "" {
		args = append(args, "description="+description)
	}
	args = append(args, "-o", "json")
	raw, err := c.run(args)
	if err != nil {
		return Secret{}, err
	}
	var s Secret
	if err := json.Unmarshal(raw, &s); err != nil {
		return Secret{}, fmt.Errorf("parse scw secret create: %w", err)
	}
	c.cachedIdx = nil
	return s, nil
}

func (c *Client) PushVersion(secretID string, data []byte) (int, error) {
	encoded := base64.StdEncoding.EncodeToString(data)
	raw, err := c.run([]string{"secret", "version", "create",
		"secret-id=" + secretID,
		"data=" + encoded,
		"-o", "json",
	})
	if err != nil {
		return 0, err
	}
	var v Version
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, fmt.Errorf("parse scw version create: %w", err)
	}
	c.cachedIdx = nil
	return v.Revision, nil
}

func (c *Client) DeleteSecret(secretID string) error {
	_, err := c.run([]string{"secret", "secret", "delete", "secret-id=" + secretID})
	c.cachedIdx = nil
	return err
}

// AccountProjectGet returns nil if the project ID resolves under the
// current SCW credentials. Returns an error otherwise.
func (c *Client) AccountProjectGet(projectID string) error {
	_, err := c.run([]string{"account", "project", "get", "project-id=" + projectID})
	return err
}

// DNSZone mirrors one entry from `scw dns zone list -o json`. We only
// care about the apex-zone shape (subdomain == "") + the NS list to
// confirm the zone is delegated to scaleway.
type DNSZone struct {
	Domain    string   `json:"domain"`
	Subdomain string   `json:"subdomain"`
	NS        []string `json:"ns"`
	NSDefault []string `json:"ns_default"`
	Status    string   `json:"status"`
}

// ListDNSZones returns every DNS zone visible to the current credentials.
func (c *Client) ListDNSZones() ([]DNSZone, error) {
	raw, err := c.run([]string{"dns", "zone", "list", "-o", "json"})
	if err != nil {
		return nil, err
	}
	var zones []DNSZone
	if err := json.Unmarshal(raw, &zones); err != nil {
		return nil, fmt.Errorf("parse scw dns zone list: %w", err)
	}
	return zones, nil
}

// FindApexZone returns the apex zone (subdomain == "") matching `domain`.
// Returns nil + nil if not found, error on transport failure.
func (c *Client) FindApexZone(domain string) (*DNSZone, error) {
	zones, err := c.ListDNSZones()
	if err != nil {
		return nil, err
	}
	for i := range zones {
		z := &zones[i]
		if z.Subdomain == "" && z.Domain == domain {
			return z, nil
		}
	}
	return nil, nil
}

// IsDelegatedToScaleway reports whether the zone's authoritative
// nameservers point at scaleway. NS values may include trailing dots —
// strip before comparing.
func IsDelegatedToScaleway(z *DNSZone) bool {
	if z == nil {
		return false
	}
	for _, ns := range append(append([]string{}, z.NS...), z.NSDefault...) {
		trimmed := ns
		if l := len(trimmed); l > 0 && trimmed[l-1] == '.' {
			trimmed = trimmed[:l-1]
		}
		if trimmed == "ns0.dom.scw.cloud" || trimmed == "ns1.dom.scw.cloud" {
			return true
		}
	}
	return false
}

// SSHKey is one entry in `scw iam ssh-key list`.
type SSHKey struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

// ListSSHKeys returns every IAM ssh key under the given project. Empty
// projectID lists across the user's whole org.
func (c *Client) ListSSHKeys(projectID string) ([]SSHKey, error) {
	args := []string{"iam", "ssh-key", "list", "-o", "json"}
	if projectID != "" {
		args = append(args, "project-id="+projectID)
	}
	raw, err := c.run(args)
	if err != nil {
		return nil, err
	}
	var keys []SSHKey
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil, fmt.Errorf("parse scw iam ssh-key list: %w", err)
	}
	return keys, nil
}

// EnsureSSHKey is idempotent — adds the given public key to the project's
// IAM keystore if its body (first two whitespace-separated fields, eg.
// `ssh-ed25519 AAAA...`) doesn't already appear there. Returns nil if a
// match exists or the upload succeeds.
func (c *Client) EnsureSSHKey(name, pubKeyContent, projectID string) error {
	body := keyBody(pubKeyContent)
	if body == "" {
		return fmt.Errorf("invalid public key content")
	}
	existing, err := c.ListSSHKeys(projectID)
	if err != nil {
		return err
	}
	for _, k := range existing {
		if keyBody(k.PublicKey) == body {
			return nil
		}
	}
	args := []string{
		"iam", "ssh-key", "create",
		"name=" + name,
		"public-key=" + pubKeyContent,
	}
	if projectID != "" {
		args = append(args, "project-id="+projectID)
	}
	_, err = c.run(args)
	return err
}

// keyBody strips the comment field from an OpenSSH-format public key:
// "ssh-ed25519 AAAA... user@host" → "ssh-ed25519 AAAA...". Robust to
// re-uploads from machines whose hostname has changed.
func keyBody(content string) string {
	content = strings.TrimSpace(content)
	parts := strings.Fields(content)
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + " " + parts[1]
}

// BucketExists returns true if a bucket with the given name exists in the
// given region under the current SCW_DEFAULT_PROJECT_ID.
func (c *Client) BucketExists(name, region string) (bool, error) {
	args := []string{"object", "bucket", "list", "-o", "json"}
	if region != "" {
		args = append(args, "region="+region)
	}
	raw, err := c.run(args)
	if err != nil {
		return false, err
	}
	var buckets []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &buckets); err != nil {
		return false, fmt.Errorf("parse scw object bucket list: %w", err)
	}
	for _, b := range buckets {
		if b.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// CreateBucket creates an S3 bucket configured for terraform state:
// versioning enabled (insurance against `terraform state rm` accidents)
// and acl=private (no public read — scw's canonical private setting).
// No-op + nil error if a bucket with this name already exists.
func (c *Client) CreateBucket(name, region string) error {
	if len(name) > 63 {
		return fmt.Errorf("bucket name %q is %d chars; scw object storage caps at 63", name, len(name))
	}
	exists, err := c.BucketExists(name, region)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	args := []string{
		"object", "bucket", "create",
		"name=" + name,
		"acl=private",
		"enable-versioning=true",
	}
	if region != "" {
		args = append(args, "region="+region)
	}
	_, err = c.run(args)
	return err
}

func ParseRevision(s string) (string, error) {
	if s == "latest" {
		return "latest", nil
	}
	if _, err := strconv.Atoi(s); err != nil {
		return "", fmt.Errorf("invalid revision %q: must be a number or 'latest'", s)
	}
	return s, nil
}

