package docker

import (
	"reflect"
	"testing"
)

func TestArgsMinimal(t *testing.T) {
	got := Invocation{Image: "ghcr.io/sheyaln/sabokit-runner:v0.1.0"}.Args()
	want := []string{"run", "--rm", "ghcr.io/sheyaln/sabokit-runner:v0.1.0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestArgsFull(t *testing.T) {
	inv := Invocation{
		Image:      "img:v1",
		Platform:   "linux/amd64",
		Pull:       "always",
		Workdir:    "/work",
		Entrypoint: "ansible-playbook",
		Cmd:        []string{"deploy.yml", "--tags", "espocrm"},
		Mounts: []Mount{
			{Source: "/host/work", Target: "/work"},
			{Source: "/host/keys", Target: "/keys", ReadOnly: true},
		},
		Env:     map[string]string{"FOO": "bar", "BAZ": "qux"},
		TTY:     true,
		NetHost: true,
	}
	got := inv.Args()
	want := []string{
		"run", "--rm",
		"--platform", "linux/amd64",
		"--pull", "always",
		"-it", "--network", "host",
		"-w", "/work",
		"-v", "/host/work:/work",
		"-v", "/host/keys:/keys:ro",
		"-e", "BAZ=qux",
		"-e", "FOO=bar",
		"--entrypoint", "ansible-playbook",
		"img:v1",
		"deploy.yml", "--tags", "espocrm",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
