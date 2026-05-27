package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var servers []string
	var tail int
	var follow bool
	var container string
	var group string
	cmd := &cobra.Command{
		Use:   "logs <app>",
		Short: "docker logs via SSH (container name defaults to app name)",
		Long: `runs 'ssh <user>@<host> -- docker logs [-f] --tail N <container>' against
the host that runs <app>.

host resolution:
  --servers <host>   explicit; bypasses inventory
  otherwise          reads environments/<env>/inventory.ini and picks the
                     single host in the --group group (default: apps).
                     errors if 0 or >1 hosts resolve — pass --servers to
                     disambiguate, or set a different --group (eg.
                     'identity' for authentik).

container name defaults to the app name. override with --container if
your deployment uses a different name.

does not require docker locally — ssh + remote docker only.`,
		Example: `  sabokit logs espocrm
  sabokit logs espocrm -f --tail 500
  sabokit logs authentik --group identity
  sabokit logs n8n --servers app01-prod
  sabokit logs n8n --container n8n-worker-1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(args[0], servers, container, group, tail, follow)
		},
	}
	cmd.Flags().StringSliceVar(&servers, "servers", nil, "host to ssh into (bypasses inventory lookup)")
	cmd.Flags().IntVar(&tail, "tail", 100, "lines to tail")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow log output")
	cmd.Flags().StringVar(&container, "container", "", "container name (default: <app>)")
	cmd.Flags().StringVar(&group, "group", "apps", "ansible group to resolve host from (used when --servers not set)")
	return cmd
}

func runLogs(app string, servers []string, container, group string, tail int, follow bool) error {
	p, err := project.Load()
	if err != nil {
		return err
	}

	hosts := servers
	if len(hosts) == 0 {
		hosts, err = resolveHostsFromInventory(p, group)
		if err != nil {
			return err
		}
	}
	if len(hosts) == 0 {
		return fmt.Errorf("no host resolved for app %q in group %q — pass --servers to pick one", app, group)
	}
	if len(hosts) > 1 {
		return fmt.Errorf("multiple hosts in group %q: %v — pass --servers to disambiguate", group, hosts)
	}
	host := hosts[0]

	if container == "" {
		container = app
	}
	user := p.Config.SSH.User
	if user == "" {
		user = "root"
	}

	dockerCmd := []string{"docker", "logs", "--tail", strconv.Itoa(tail)}
	if follow {
		dockerCmd = append(dockerCmd, "-f")
	}
	dockerCmd = append(dockerCmd, container)

	sshArgs := []string{}
	if key := p.Config.SSH.Key; key != "" {
		sshArgs = append(sshArgs, "-i", expandHome(key))
	}
	sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", user, host), "--")
	sshArgs = append(sshArgs, dockerCmd...)

	c := exec.Command("ssh", sshArgs...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func resolveHostsFromInventory(p *project.Project, group string) ([]string, error) {
	if p.EnvName(globals.Env) == "" {
		return nil, fmt.Errorf("no env set — pass --env, set default_env, or pass --servers explicitly")
	}
	return p.InventoryHosts(globals.Env, group)
}
