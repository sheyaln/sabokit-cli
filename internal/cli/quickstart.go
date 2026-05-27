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

1. scaffold a project + first env in one shot
   --------------------------------------------
   sabokit init my-stack --env prod
   #   prompts for base domain, scaleway region, ssh user/key
   #   --non-interactive --base-domain example.com to skip prompts

   # already have a consumer-template checkout? generate just the config:
   #   cd existing-repo && sabokit config init

   cd my-stack

   you now have:
     .sabokit/config.yml             (sabokit-cli scope, default_env: prod)
     apps-manifest.yaml              (catalog from consumer-template)
     environments/_template/         (copy this per new env)
     environments/prod/              (your bootstrapped env)
       config.tf.example             (edit + rename → config.tf)
       backend.hcl.example
       inventory.ini.example
       preflight.sh, up.sh, configure.sh, secrets.tf
     modules/stack/                  (shared TF wiring)

2. set scaleway credentials
   -------------------------
   export SCW_ACCESS_KEY=...
   export SCW_SECRET_KEY=...
   export SCW_DEFAULT_PROJECT_ID=...
   export SCW_DEFAULT_REGION=fr-par
   export SCW_DEFAULT_ZONE=fr-par-1

3. arm64 hosts: set the platform (runner image is amd64-only)
   ----------------------------------------------------------
   export SABOKIT_PLATFORM=linux/amd64
   # default --image is ghcr.io/sheyaln/sabokit-runner (latest stable);
   # override with SABOKIT_IMAGE=...:vX.Y.Z if you need a specific tag.

4. fill in the env config
   -----------------------
   cd environments/prod
   cp config.tf.example     config.tf      && $EDITOR config.tf
   cp backend.hcl.example   backend.hcl    && $EDITOR backend.hcl
   cp inventory.ini.example inventory.ini   # placeholder; sabokit up rewrites
   cd ../..

5. provision the env (terraform + ansible bootstrap + Authentik configure)
   -----------------------------------------------------------------------
   sabokit up   # pure-Go orchestration:
                #   tf apply → inventory regen → ssh wait → ansible bootstrap
                #   → LE cert wait → blueprint indexing → tf apply (full)
                # uses hashicorp/terraform:1.9, sabokit-runner, scaleway/cli
                # only host requirements: docker + ssh
                # --skip-up / --skip-configure for re-runs

6. operate (env is auto from .sabokit/config.yml's default_env)
   ------------------------------------------------------------
   sabokit apps list                        # catalog (what's available)
   sabokit apps list --enabled              # what's running in current env
   sabokit apps add vikunja                 # enable in config.tf
   sabokit apps remove jitsi                # disable in config.tf
   sabokit deploy --apps espocrm --check    # apps.yml --check (dry run)
   sabokit deploy --apps espocrm            # apps.yml for real
   sabokit deploy --base                    # site.yml (bootstrap + apps)
   sabokit status                           # tf-output.json + docker ps
   sabokit logs espocrm -f                  # follow container logs (apps group)
   sabokit logs authentik --group identity  # different ansible group
   sabokit ssh app01-prod                   # shell into the host
   sabokit down --apps espocrm              # stop the containers
   sabokit destroy --apps espocrm           # terraform destroy -target=...
   sabokit destroy --all                    # terraform destroy (entire env)

   # override env per-call:
   sabokit --env staging deploy --apps espocrm

7. inspect or rotate secrets (independent of runner image)
   -------------------------------------------------------
   sabokit secrets list                          # clean 4-col table
   sabokit secrets list --tag authentik
   sabokit secrets get db-password               # plaintext to stdout
   sabokit secrets rotate db-pw 'new' --apps espocrm
                                                  # push version + redeploy

troubleshooting:
  - "no .sabokit/config.yml found"                → cd into your project dir
  - "environment X not found"                     → check environments/<X>/ exists
  - "no .tf-output.json"                          → run up.sh in the env dir first
  - "docker daemon not reachable"                 → start docker desktop / dockerd
  - "ssh permission denied"                       → ssh-add your key
  - "no matching manifest" on docker pull         → export SABOKIT_PLATFORM=linux/amd64

see 'sabokit <command> --help' for full flag detail.
`
