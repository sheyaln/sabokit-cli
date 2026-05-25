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
		Args:  cobra.ExactArgs(1),
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
		user = "root"
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
