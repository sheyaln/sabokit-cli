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
	cmd := &cobra.Command{
		Use:   "logs <app>",
		Short: "docker logs via SSH (container name defaults to app name)",
		Long: `runs 'ssh <user>@<host> -- docker logs [-f] --tail N <container>' against
the host that runs <app>.

host resolution:
  if --servers is set, uses that host(s) directly
  otherwise reads apps-manifest.yaml and uses the app's 'host' field
  errors if 0 or >1 hosts resolve — pass --servers to disambiguate

container name defaults to the app name. override with --container if your
deployment uses a different container name (eg. when docker compose
suffixes the service name).

does not require docker locally — ssh + remote docker only.`,
		Example: `  # last 100 lines (default)
  sabokit logs espocrm

  # follow live, 500 lines of backfill
  sabokit logs espocrm --tail 500 -f

  # pick the host explicitly
  sabokit logs authentik --servers app02

  # override the container name
  sabokit logs n8n --container n8n-worker-1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(args[0], servers, container, tail, follow)
		},
	}
	cmd.Flags().StringSliceVar(&servers, "servers", nil, "host to ssh into (overrides app's host from manifest)")
	cmd.Flags().IntVar(&tail, "tail", 100, "lines to tail")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow log output")
	cmd.Flags().StringVar(&container, "container", "", "container name (default: <app>)")
	return cmd
}

func runLogs(app string, servers []string, container string, tail int, follow bool) error {
	p, err := project.Load()
	if err != nil {
		return err
	}

	hosts := servers
	if len(hosts) == 0 {
		hosts, err = p.HostsForApp(app)
		if err != nil {
			return err
		}
	}
	if len(hosts) == 0 {
		return fmt.Errorf("no host resolved for app %q — set 'host' in apps-manifest or pass --servers", app)
	}
	if len(hosts) > 1 {
		return fmt.Errorf("app %q maps to multiple hosts %v — pass --servers to pick one", app, hosts)
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
