package docker

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
)

type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

type Invocation struct {
	Image       string
	Workdir     string
	Entrypoint  string
	Cmd         []string
	Mounts      []Mount
	Env         map[string]string
	EnvPassthru []string
	TTY         bool
	NetHost     bool
}

func (i Invocation) Args() []string {
	args := []string{"run", "--rm"}
	if i.TTY {
		args = append(args, "-it")
	}
	if i.NetHost {
		args = append(args, "--network", "host")
	}
	if i.Workdir != "" {
		args = append(args, "-w", i.Workdir)
	}
	for _, m := range i.Mounts {
		spec := fmt.Sprintf("%s:%s", m.Source, m.Target)
		if m.ReadOnly {
			spec += ":ro"
		}
		args = append(args, "-v", spec)
	}

	keys := make([]string, 0, len(i.Env))
	for k := range i.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, i.Env[k]))
	}
	for _, k := range i.EnvPassthru {
		if _, ok := os.LookupEnv(k); ok {
			args = append(args, "-e", k)
		}
	}

	if i.Entrypoint != "" {
		args = append(args, "--entrypoint", i.Entrypoint)
	}
	args = append(args, i.Image)
	args = append(args, i.Cmd...)
	return args
}

func Preflight() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found in PATH — install Docker Desktop or the docker CLI")
	}
	c := exec.Command("docker", "info")
	c.Stdout = nil
	c.Stderr = nil
	if err := c.Run(); err != nil {
		return fmt.Errorf("docker daemon not reachable — is Docker running? (docker info failed: %w)", err)
	}
	return nil
}

func (i Invocation) Command() *exec.Cmd {
	c := exec.Command("docker", i.Args()...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c
}
