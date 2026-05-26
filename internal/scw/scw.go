package scw

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

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

func ParseRevision(s string) (string, error) {
	if s == "latest" {
		return "latest", nil
	}
	if _, err := strconv.Atoi(s); err != nil {
		return "", fmt.Errorf("invalid revision %q: must be a number or 'latest'", s)
	}
	return s, nil
}

