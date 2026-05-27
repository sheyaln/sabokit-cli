package cli

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/spf13/cobra"
)

func newSSHCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ssh <host>",
		Short: "ssh into a managed host",
		Long: `passthrough to 'ssh <user>@<host>'. user comes from .sabokit/config.yml
(ssh.user, defaults to root); the optional ssh.key is passed via -i.

sabokit does not consult the inventory; <host> is whatever ssh can resolve
— a hostname, an alias from your ~/.ssh/config, or a raw IP. uses
syscall.Exec so your local shell is replaced by the ssh session (no
sabokit process lingers in the parent chain).

does not require docker.`,
		Example: `  sabokit ssh app01
  sabokit ssh edge.example.com`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSSH(args[0])
		},
	}
}

func runSSH(host string) error {
	p, err := project.Load()
	if err != nil {
		return err
	}
	user := p.Config.SSH.User
	if user == "" {
		user = "ubuntu"
	}

	bin, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found in PATH: %w", err)
	}
	sshArgs := []string{"ssh"}
	if key := p.Config.SSH.Key; key != "" {
		sshArgs = append(sshArgs, "-i", expandHome(key))
	}
	sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", user, host))
	return syscall.Exec(bin, sshArgs, os.Environ())
}
