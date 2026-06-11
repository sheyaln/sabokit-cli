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

1. set scaleway credentials
   -------------------------
   export SCW_ACCESS_KEY=...
   export SCW_SECRET_KEY=...
   export SCW_DEFAULT_PROJECT_ID=...
   export SCW_DEFAULT_REGION=fr-par
   export SCW_DEFAULT_ZONE=fr-par-1

2. scaffold project + envs + state buckets in one shot
   ---------------------------------------------------
   sabokit init my-stack
   #   interactive: prompts for base_domain, project UUID, org_slug,
   #     org_name, infra_email, ssh user/key, first env name, staging y/n
   #   non-interactive: pass --base-domain --scaleway-project-id
   #     --org-slug --org-name --infra-email --env (--staging)
   #
   #   sabokit clones consumer-template, scaffolds environments/<env>/
   #   (env.yml + per-layer backend.hcl pre-filled) and creates the
   #   scaleway object bucket each env's TF state will live in.

   cd my-stack

   # review the per-layer YAML — apps, tiers, hosts, watchers
   $EDITOR environments/prod/application.yml

3. provision + deploy the env end-to-end — one command
   ----------------------------------------------------
   sabokit up   # preflight (creds, ssh key in IAM, DNS zone, backends,
                #   state bucket) + one confirmation, then the four layer
                #   scripts inside the runner image, in order:
                #   infra → identity → operations → application
                # the identity layer waits for SSH, DNS propagation, the
                #   Let's Encrypt cert, and Authentik indexing by itself
                # only host requirements: docker + ssh + git
                # idempotent — re-run after a failure and it resumes
                # --layers application for a single layer
                # --no-confirm for unattended runs

4. operate (env is auto from .sabokit/config.yml's default_env)
   ------------------------------------------------------------
   sabokit apps list                        # catalog (what's available)
   sabokit apps list --enabled              # what's running in current env
   sabokit apps add vikunja                 # enable in application.yml
   sabokit apps remove jitsi                # disable in application.yml
   sabokit up --layers application          # apply app changes (tf + ansible)
   sabokit deploy --apps espocrm --check    # ansible-only redeploy (dry run)
   sabokit deploy --apps espocrm            # ansible-only redeploy
   sabokit deploy --all                     # bootstrap + everything
   sabokit refresh                          # regen inventory + enabled_apps
   sabokit status                           # enabled apps + docker ps
   sabokit logs espocrm -f                  # follow container logs (apps group)
   sabokit logs authentik --group identity  # different ansible group
   sabokit ssh app01-prod                   # shell into the host
   sabokit down --apps espocrm              # stop the containers
   sabokit destroy --layer application      # terraform destroy one layer
   sabokit destroy --all                    # tear down the entire env

   # override env per-call:
   sabokit --env staging up

5. inspect or rotate secrets (independent of runner image)
   -------------------------------------------------------
   sabokit secrets list                          # clean 4-col table
   sabokit secrets list --tag authentik
   sabokit secrets get db-password               # plaintext to stdout
   sabokit secrets rotate db-pw 'new' --apps espocrm
                                                  # push version + redeploy

troubleshooting:
  - "no .sabokit/config.yml found"                → cd into your project dir
  - "environment X not found"                     → check environments/<X>/ exists
  - "no .enabled_apps.json"                       → run 'sabokit refresh'
  - "docker daemon not reachable"                 → start docker desktop / dockerd
  - "ssh permission denied"                       → ssh-add your key
  - "no matching manifest" on docker pull         → image lacks your arch; force one with SABOKIT_PLATFORM=linux/amd64

see 'sabokit <command> --help' for full flag detail.
`
