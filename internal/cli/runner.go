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
	// containerRepo is where the whole consumer repo is mounted. The runner
	// image bakes /workspace/sabokit -> /opt/sabokit, so the consumer's
	// ansible-local/site.yml resolves its ../../sabokit sibling import.
	containerRepo = "/workspace/consumer"

	// bakedScriptsDir holds the consumer-template layer scripts baked into
	// the runner image at the matching blueprint tag — the fallback when the
	// consumer hasn't vendored scripts/ itself.
	bakedScriptsDir = "/opt/sabokit/consumer-template/scripts"

	playbookDir = "/opt/sabokit/platform/ansible"
)

// repoInvocation mounts the consumer repo at /workspace/consumer with the
// SSH agent and Scaleway credentials passed through — the base for every
// layer-script and ansible run.
func repoInvocation(p *project.Project) (docker.Invocation, error) {
	image, err := runnerImage(p)
	if err != nil {
		return docker.Invocation{}, err
	}
	mounts := []docker.Mount{
		{Source: p.Root, Target: containerRepo},
	}
	env := map[string]string{}
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		mounts = append(mounts, docker.Mount{Source: sock, Target: "/ssh-agent"})
		env["SSH_AUTH_SOCK"] = "/ssh-agent"
	}
	if p.Config.SSH.Key != "" {
		expanded := expandHome(p.Config.SSH.Key)
		if _, err := os.Stat(expanded); err == nil {
			mounts = append(mounts, docker.Mount{Source: expanded, Target: "/keys/ssh_key", ReadOnly: true})
			env["ANSIBLE_PRIVATE_KEY_FILE"] = "/keys/ssh_key"
		}
	}
	return docker.Invocation{
		Image:       image,
		Platform:    globals.Platform,
		Pull:        globals.Pull,
		Workdir:     containerRepo,
		Mounts:      mounts,
		Env:         env,
		EnvPassthru: []string{"SCW_ACCESS_KEY", "SCW_SECRET_KEY", "SCW_DEFAULT_PROJECT_ID", "SCW_DEFAULT_ORGANIZATION_ID", "SCW_DEFAULT_REGION", "SCW_DEFAULT_ZONE"},
		TTY:         isTerminal(),
	}, nil
}

// scriptPath resolves a layer script to its in-container path: the
// consumer's own scripts/<name> when vendored, else the baked
// consumer-template copy (which matches the env's pinned blueprint tag by
// construction — the runner image tag follows the pin).
func scriptPath(p *project.Project, name string) string {
	if _, err := os.Stat(filepath.Join(p.Root, "scripts", name)); err == nil {
		return "scripts/" + name
	}
	return bakedScriptsDir + "/" + name
}

// scriptInvocation builds the docker run for one layer script.
func scriptInvocation(p *project.Project, name string, args ...string) (docker.Invocation, error) {
	inv, err := repoInvocation(p)
	if err != nil {
		return docker.Invocation{}, err
	}
	inv.Entrypoint = "bash"
	inv.Cmd = append([]string{scriptPath(p, name)}, args...)
	return inv, nil
}

// runScript executes one layer script inside the runner.
func runScript(p *project.Project, name string, args ...string) error {
	inv, err := scriptInvocation(p, name, args...)
	if err != nil {
		return err
	}
	return inv.Command().Run()
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
