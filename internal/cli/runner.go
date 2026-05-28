package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sheyaln/sabokit-cli/internal/docker"
	"github.com/sheyaln/sabokit-cli/internal/project"
)

const (
	containerWorkspace = "/workspace"
	playbookDir        = "/opt/sabokit/platform/ansible"
	terraformDir       = "/opt/sabokit/platform/terraform"
)

func baseInvocation(p *project.Project) (docker.Invocation, error) {
	workspace, err := p.WorkspaceDir(globals.Env)
	if err != nil {
		return docker.Invocation{}, err
	}
	image, err := runnerImage(p)
	if err != nil {
		return docker.Invocation{}, err
	}
	mounts := []docker.Mount{
		{Source: workspace, Target: containerWorkspace},
	}
	env := map[string]string{}
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		mounts = append(mounts, docker.Mount{Source: sock, Target: "/ssh-agent"})
		env["SSH_AUTH_SOCK"] = "/ssh-agent"
	}
	if p.Config.SSH.Key != "" {
		expanded := expandHome(p.Config.SSH.Key)
		mounts = append(mounts, docker.Mount{Source: expanded, Target: "/keys/ssh_key", ReadOnly: true})
		env["SABOKIT_SSH_KEY"] = "/keys/ssh_key"
	}
	return docker.Invocation{
		Image:       image,
		Platform:    globals.Platform,
		Mounts:      mounts,
		Env:         env,
		EnvPassthru: []string{"SCW_ACCESS_KEY", "SCW_SECRET_KEY", "SCW_DEFAULT_PROJECT_ID", "SCW_DEFAULT_ORGANIZATION_ID", "SCW_DEFAULT_REGION", "SCW_DEFAULT_ZONE"},
		TTY:         isTerminal(),
	}, nil
}

// runnerImage returns the sabokit-runner image ref. An explicit
// --image / SABOKIT_IMAGE wins; otherwise the tag follows the environment's
// pinned sabokit version so the ansible half matches the terraform half.
func runnerImage(p *project.Project) (string, error) {
	if globals.Image != "" {
		return globals.Image, nil
	}
	ref, err := p.BlueprintVersion(globals.Env)
	if err != nil {
		return "", fmt.Errorf("resolve runner image from env pin: %w (pass --image or set SABOKIT_IMAGE)", err)
	}
	return DefaultRunnerImage + ":" + ref, nil
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
