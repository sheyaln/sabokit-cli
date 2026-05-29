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

   # arm64 hosts: the runner image is amd64-only
   export SABOKIT_PLATFORM=linux/amd64

2. scaffold project + envs + state buckets in one shot
   ---------------------------------------------------
   sabokit init my-stack
   #   interactive: prompts for base_domain, project UUID, org_slug,
   #     org_name, infra_email, ssh user/key, first env name, staging y/n
   #   non-interactive: pass --base-domain --scaleway-project-id
   #     --org-slug --org-name --infra-email --env (--staging)
   #
   #   sabokit clones consumer-template, scaffolds environments/<env>/
   #   with config.tf + backend.hcl pre-filled, and creates the scaleway
   #   object bucket each env's TF state will live in.

   cd my-stack

   # edit config.tf if you need anything beyond the prompted scalars
   # (compute_hosts, identity tier_slots, per-app blocks)
   $EDITOR environments/prod/config.tf

3. provision the env end-to-end
   ----------------------------
   sabokit up   # pure-Go orchestration:
                #   preflight (config + creds + ssh key in IAM)
                #   tf apply (base + identity_bootstrap)
                #   inventory regen, ssh wait, ansible bootstrap
                #   LE cert wait, blueprint indexing wait
                #   full tf apply (identity + apps)
                # uses hashicorp/terraform:1.13, sabokit-runner, scaleway/cli
                # only host requirements: docker + ssh + git
                # --skip-preflight / --skip-up / --skip-configure for re-runs

4. operate (env is auto from .sabokit/config.yml's default_env)
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
  - "no .tf-output.json"                          → run up.sh in the env dir first
  - "docker daemon not reachable"                 → start docker desktop / dockerd
  - "ssh permission denied"                       → ssh-add your key
  - "no matching manifest" on docker pull         → export SABOKIT_PLATFORM=linux/amd64

see 'sabokit <command> --help' for full flag detail.
`
