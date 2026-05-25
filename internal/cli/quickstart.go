package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newQuickstartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "quickstart",
		Short: "print a step-by-step walkthrough for using v0.1.0 against a real project",
		Long: `prints the manual setup + first-deploy walkthrough. 'sabokit init' will
collapse most of this into one command once implemented; until then, this
is the on-ramp.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print(quickstartText)
			return nil
		},
	}
}

const quickstartText = `sabokit quickstart
==================

prerequisites:
  - docker (daemon running)
  - ssh (key in agent or filesystem)
  - a scaleway account with API keys

1. create the project directory
   -----------------------------
   mkdir my-stack && cd my-stack
   mkdir .sabokit

2. write .sabokit/config.yml
   --------------------------
   project: my-stack
   scaleway:
     region: fr-par
     zone: fr-par-1
   ssh:
     user: root
     key: ~/.ssh/id_ed25519

3. write apps-manifest.yaml
   -------------------------
   apps:
     espocrm:
       enabled: true
       host: app01
     authentik:
       enabled: true
       host: app02

4. write inventory.ini
   --------------------
   (real inventory comes from your terraform outputs; for now a stub is fine
    to test the cli surface)

   [all]
   app01 ansible_host=51.x.x.x
   app02 ansible_host=51.y.y.y

5. set scaleway credentials in the environment
   --------------------------------------------
   export SCW_ACCESS_KEY=...
   export SCW_SECRET_KEY=...
   export SCW_DEFAULT_PROJECT_ID=...
   export SCW_DEFAULT_REGION=fr-par
   export SCW_DEFAULT_ZONE=fr-par-1

6. verify your config parses
   --------------------------
   sabokit apps list
   sabokit apps list --enabled

7. pick a runner image
   --------------------
   v0.1.0 defaults --image to ghcr.io/sheyaln/sabokit-runner:v3.0.0, which
   does not exist yet. until it ships, point at the older runner:

   alias sabokit='sabokit --image ghcr.io/sheyaln/federated-commons-runner:v2.17.0'

   (only 'deploy' works against v2.17.0. 'down' and parts of 'status' need
    v3.0.0 to land first.)

8. see the docker invocation without running it
   ---------------------------------------------
   sabokit deploy --apps espocrm --servers app01 --print

   this prints the full 'docker run ...' command sabokit would execute.
   useful for understanding what's happening before letting it run.

9. dry-run the deploy (ansible --check)
   ------------------------------------
   sabokit deploy --apps espocrm --servers app01 --check

   nothing changes; you see what ansible would do.

10. real deploy
    -----------
    sabokit deploy --apps espocrm --servers app01

11. operate
    -------
    sabokit status                     # tf outputs + docker ps
    sabokit logs espocrm -f            # follow container logs
    sabokit ssh app01                  # shell into the host
    sabokit down --apps espocrm        # stop the containers (needs v3.0.0)

troubleshooting:
  - "no .sabokit/config.yml found"  → you're not inside the project dir
  - "docker daemon not reachable"   → start docker desktop / dockerd
  - "manifest read errors"          → check yaml indentation in apps-manifest.yaml
  - "ssh permission denied"         → ssh-add your key, or set ssh.key in config.yml

see 'sabokit <command> --help' for full flag detail on each subcommand.
`
