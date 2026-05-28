// Package tf wraps the terraform CLI by running hashicorp's official
// image via docker. sabokit-cli must never assume terraform is installed
// on the host; the entire orchestration runs through containers.
package tf

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
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
	"TF_VAR_scaleway_access_key",
	"TF_VAR_scaleway_secret_key",
}

type Client struct {
	image       string
	platform    string
	projectRoot string
}

// New builds a terraform client. projectRoot (the consumer repo root) is
// bind-mounted at /workspace so the container sees the same directory layout a
// host `terraform` run sees — relative module sources (../../modules/stack)
// and the keyed ../env-values.yml resolve identically. Pass project.Load().Root.
func New(image, platform, projectRoot string) *Client {
	return &Client{image: image, platform: platform, projectRoot: projectRoot}
}

// Workspace is the in-container path the env dir is mounted at.
const Workspace = "/workspace"

func (c *Client) invocation(workdir string, args ...string) docker.Invocation {
	return docker.Invocation{
		Image:       c.image,
		Platform:    c.platform,
		Workdir:     workdir,
		Entrypoint:  "terraform",
		Cmd:         args,
		EnvPassthru: envPassthru,
		TTY:         false,
	}
}

// withEnvDir mounts the whole project root at /workspace and sets the workdir
// to the env subdir, so the container sees the exact layout a host terraform
// run sees: relative module sources (../../modules/stack) and the keyed
// ../env-values.yml resolve identically, and the CLI runs the same terraform a
// hand-run would. envDir is <root>/environments/<env>, or the root itself in
// flat-layout mode (workdir collapses to /workspace).
func (c *Client) withEnvDir(envDir string, args ...string) docker.Invocation {
	rel, err := filepath.Rel(c.projectRoot, envDir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		// envDir not under projectRoot (shouldn't happen) — mount it alone.
		inv := c.invocation(Workspace, args...)
		inv.Mounts = []docker.Mount{{Source: envDir, Target: Workspace}}
		return inv
	}
	inv := c.invocation(path.Join(Workspace, filepath.ToSlash(rel)), args...)
	inv.Mounts = []docker.Mount{{Source: c.projectRoot, Target: Workspace}}
	return inv
}

// run executes the invocation, streaming stdout/stderr to the user.
func (c *Client) run(envDir string, args []string) error {
	inv := c.withEnvDir(envDir, args...)
	cmd := inv.Command()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// capture executes the invocation and captures stdout (used for outputs).
func (c *Client) capture(envDir string, args []string) ([]byte, error) {
	inv := c.withEnvDir(envDir, args...)
	var stdout, stderr bytes.Buffer
	cmd := inv.Command()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("terraform %v: %w\n%s", args, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// Init runs `terraform init -backend-config=<file>`. backendConfig is the
// filename within the env dir (eg "backend.hcl"). Idempotent — safe to
// call before every apply.
func (c *Client) Init(envDir, backendConfig string) error {
	args := []string{"init", "-input=false"}
	if backendConfig != "" {
		args = append(args, "-backend-config="+backendConfig)
	}
	return c.run(envDir, args)
}

type ApplyOpts struct {
	Targets     []string
	Vars        map[string]string
	Parallelism int
	AutoApprove bool
}

// Apply runs `terraform apply` with the given options.
func (c *Client) Apply(envDir string, opts ApplyOpts) error {
	args := []string{"apply", "-input=false"}
	if opts.AutoApprove {
		args = append(args, "-auto-approve")
	}
	if opts.Parallelism > 0 {
		args = append(args, fmt.Sprintf("-parallelism=%d", opts.Parallelism))
	}
	for _, t := range opts.Targets {
		args = append(args, "-target="+t)
	}
	for k, v := range opts.Vars {
		args = append(args, "-var", k+"="+v)
	}
	return c.run(envDir, args)
}

// Output runs `terraform output -json` and returns the raw bytes.
func (c *Client) Output(envDir string) ([]byte, error) {
	return c.capture(envDir, []string{"output", "-json"})
}

// PlanFile is the saved plan path, relative to the workdir (the env dir), so
// plan -out and apply land in the same place regardless of mount layout.
const PlanFile = ".tfplan"

// Plan runs `terraform plan -out=<PlanFile>` with the same option shape
// as Apply. The user sees streamed plan output; the binary plan is
// written to .tfplan inside the env dir and consumed by ApplyPlan.
func (c *Client) Plan(envDir string, opts ApplyOpts) error {
	args := []string{"plan", "-input=false", "-out=" + PlanFile}
	if opts.Parallelism > 0 {
		args = append(args, fmt.Sprintf("-parallelism=%d", opts.Parallelism))
	}
	for _, t := range opts.Targets {
		args = append(args, "-target="+t)
	}
	for k, v := range opts.Vars {
		args = append(args, "-var", k+"="+v)
	}
	return c.run(envDir, args)
}

// ApplyPlan applies a previously-saved plan file. Options were baked in
// at plan time so no re-passing is needed.
func (c *Client) ApplyPlan(envDir string) error {
	return c.run(envDir, []string{"apply", "-input=false", PlanFile})
}

type ImportOpts struct {
	Vars map[string]string
}

// Import runs `terraform import <addr> <id>`. Returns nil on success or
// when the address is already in state (silent no-op for re-runs).
func (c *Client) Import(envDir, addr, id string, opts ImportOpts) error {
	args := []string{"import", "-input=false"}
	for k, v := range opts.Vars {
		args = append(args, "-var", k+"="+v)
	}
	args = append(args, addr, id)
	inv := c.withEnvDir(envDir, args...)
	var stderr bytes.Buffer
	cmd := inv.Command()
	cmd.Stdout = nil
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return nil
	}
	// "Resource already managed by Terraform" is success for our purposes.
	if bytes.Contains(stderr.Bytes(), []byte("already managed by Terraform")) {
		return nil
	}
	return fmt.Errorf("terraform import: %w\n%s", err, stderr.String())
}

// StateShow returns true if the given resource address is in TF state.
func (c *Client) StateShow(envDir, addr string) (bool, error) {
	inv := c.withEnvDir(envDir, "state", "show", addr)
	cmd := inv.Command()
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		// non-zero exit means resource is not in state
		return false, nil
	}
	return true, nil
}
