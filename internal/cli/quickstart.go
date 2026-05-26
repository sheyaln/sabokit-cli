package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newQuickstartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "quickstart",
		Short: "print a step-by-step walkthrough for using sabokit against a real project",
		Long: `prints the init + first-deploy walkthrough. follow it top-to-bottom on
a fresh machine to land your first stack.`,
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
  - git (for 'sabokit init')
  - ssh (key in agent or filesystem)
  - a scaleway account with API keys

1. scaffold a new project from consumer-template
   ----------------------------------------------
   sabokit init my-stack
   #   prompts for base domain, scaleway region, ssh user/key
   #   or pass --non-interactive --base-domain example.com [--region nl-ams]

   cd my-stack

   you now have:
     .sabokit/config.yml        (sabokit-cli scope)
     apps-manifest.yaml         (catalog of every shipped app)
     environments/_template/    (copy this per env)
     modules/stack/             (shared TF wiring)
     scripts/                   (bump-version, import-base-image)

2. set scaleway credentials
   -------------------------
   export SCW_ACCESS_KEY=...
   export SCW_SECRET_KEY=...
   export SCW_DEFAULT_PROJECT_ID=...
   export SCW_DEFAULT_REGION=fr-par
   export SCW_DEFAULT_ZONE=fr-par-1

3. pick a runner image (the default v3.0.0 is not yet published)
   -------------------------------------------------------------
   export SABOKIT_IMAGE=ghcr.io/sheyaln/federated-commons-runner:v2.17.0
   # arm64 hosts also need:
   export SABOKIT_PLATFORM=linux/amd64

4. inspect your secrets (works today, independent of the runner image)
   -------------------------------------------------------------------
   sabokit secrets list                     # clean 4-col table
   sabokit secrets list --tag authentik
   sabokit secrets get db-password          # plaintext to stdout

5. follow consumer-template's per-env flow
   ----------------------------------------
   cp -r environments/_template environments/prod
   cd environments/prod
   cp terraform.tfvars.example terraform.tfvars && $EDITOR terraform.tfvars
   cp backend.hcl.example      backend.hcl      && $EDITOR backend.hcl
   cp inventory.ini.example    inventory.ini
   ./preflight.sh && ./up.sh && ./configure.sh

6. operate (sabokit-cli wrappers work from the project root)
   ---------------------------------------------------------
   sabokit deploy --apps espocrm --check    # ansible site.yml dry-run
   sabokit deploy --apps espocrm            # for real
   sabokit status                           # tf outputs + docker ps
   sabokit logs espocrm -f                  # follow container logs
   sabokit ssh app01                        # shell into the host
   sabokit down --apps espocrm              # stop the containers
   sabokit secrets rotate db-pw 'new' --apps espocrm
                                            # push version + redeploy

note: sabokit-cli's project model currently assumes flat root layout (one
inventory.ini, one apps-manifest at the project root). consumer-template's
multi-env layout (environments/<env>/inventory.ini) doesn't fully line up
yet — that's a known gap, see the README.

troubleshooting:
  - "no .sabokit/config.yml found"  → cd into your project dir
  - "docker daemon not reachable"   → start docker desktop / dockerd
  - "git clone failed"              → check network + repo URL
  - "ssh permission denied"         → ssh-add your key, or set ssh.key in config
  - "no matching manifest"          → export SABOKIT_PLATFORM=linux/amd64

see 'sabokit <command> --help' for full flag detail on each subcommand.
`
