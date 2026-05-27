# sabokit-cli features plan

Operator-facing roadmap for what sabokit-cli absorbs from `consumer-template/environments/_template/*.sh` and what new lifecycle commands ship on top. Lives in-repo so the plan moves with the code.

## Mandate

sabokit-cli is the conductor for the deploy lifecycle and the consumer-template scaffolding. Operators never `chmod +x` shell scripts, hand-create state buckets, or `cp -r` scaffolding dirs. Terraform stays declarative. Ansible stays declarative. sabokit-cli sequences, validates, and hides two-pass complexity (Authentik bootstrap) behind one verb.

## Host requirements

`docker` + `ssh` + `git` on the host. Nothing else. terraform/ansible/scw/jq/python come from images sabokit invokes (`hashicorp/terraform:1.9`, `ghcr.io/sheyaln/sabokit-runner`, `scaleway/cli:2.56`).

## Status (as of v2026.05.0 baseline)

| Area | Shipped | Open |
| --- | --- | --- |
| `sabokit init` interactive scaffolding | prompts; project + env(s); per-env `config.tf` + `backend.hcl`; `.gitignore` + `.envrc.example` + `README.md` scaffolds; staging-default-YES; `==> ok` progress cadence | — |
| State bucket creation | per-env idempotent create via `scaleway/cli`; versioning enabled; `acl=private`; name-length (≤63) precheck | block-public-access (scw doesn't expose a flag; `acl=private` is canonical) |
| Preflight (phase 0 of `up`) | config keys + SCW creds + IAM ssh-key upload | DNS-zone-delegation check; gateway-domain `dig` propagation wait |
| `sabokit up` end-to-end | preflight + tf apply + refresh + ansible bootstrap + LE-cert wait + Authentik index wait + full apply + outpost import | `terraform plan` with human-confirm gate (prod default-confirm; `--no-confirm` opt-out) |
| Inventory + `.ansible-vars.json` regen | refreshed before every up/deploy/down/status | — |
| Two-pass Authentik | hidden behind `sabokit up` | — |
| ubuntu default ssh user | all sabokit-side fallbacks | upstream `platform/ansible/ansible.cfg` cleanup — out of scope here |
| `sabokit env add/list/switch` | — | full pack |
| `sabokit bump <tag>` | — | `?ref=` rewrites + `moved{}` migration prompt |
| `sabokit upgrade <tag>` | — | `bump` + `deploy` chain with `moved{}` gate |
| `sabokit import-base-image <tag>` | — | full pack |
| `sabokit doctor` | — | full pack |
| `sabokit cost-estimate` | — | Scaleway pricing API call |
| `sabokit version-check` | — | github tags ping |

## Ordered packs

Each row is one branch, one commit-set, one tag. No PR ceremony; merge to master and tag.

1. ~~**Foundation hardening**~~ — shipped v2026.05.1.
2. **Preflight DNS + plan-confirm gate** — `scw dns zone list` delegation check; gateway-domain Go-side `net.LookupHost` propagation wait; `terraform plan` phase with human-confirm before `apply` (default-yes when env != "prod", `--no-confirm` opt-out).
3. **`sabokit env add/list/switch`** — `env add <name>` reuses the init prompt + bucket flow for an additional env. `env list` enumerates `environments/`. `env switch <name>` rewrites `.sabokit/config.yml` `default_env`.
4. **`sabokit bump <tag>`** — rewrite every `?ref=…` pin under the consumer's TF, optionally bump a sibling sabokit submodule, prompt to run any `moved{}` migrations published in upstream's `consumer-template/modules/stack/migrations.tf` for the new tag.
5. **`sabokit upgrade <tag>`** — `bump` + `deploy` chain. Pre-apply check refuses state-mutation if any legacy address exists that isn't mapped by a `moved{}` block in the consumer's migrations.tf (backrest-bucket foot-gun).
6. **`sabokit doctor`** — state bucket reachable; terraform state file present + non-corrupt; every compute host ssh-reachable; remote docker daemon up; traefik routing healthy; certs not within expiry window; most-recent backrest run completed; loki receiving from monitoring agent.
7. **`sabokit import-base-image <tag>`** — pull `fc-base-<tag>.qcow2` from github release, upload via scaleway object storage + block snapshot + image registration, print resulting `image_id`, offer to inline-edit the relevant `compute_hosts.*.image` key in `config.tf`.
8. **`sabokit cost-estimate`** — monthly burn from `compute_hosts` shape × scaleway pricing API.
9. **`sabokit version-check`** — github tags API ping, flag if consumer is behind.

## Scaffolding non-negotiables

Two rules the upstream operator has surfaced that affect what `sabokit init` ships:

- **`.tfvars` is for secrets only.** Never scaffold a `terraform.tfvars.example` or a `terraform.tfvars` file. Operator-facing configuration (which apps are on, hostnames, image tags, host-services knobs) lives in `config.tf` as a `locals.config` block plus a `module "stack"` call. `.tfvars` is reserved for the rare case of passing plaintext secret values at apply time (gitignored by default). Don't conflate "terraform variable" with the `.tfvars` file format.
- **`inventory.ini.example` is misleading.** Upstream still ships a placeholder, but the real `inventory.ini` is generated from `terraform output -json compute_hosts` after apply. `sabokit init` should NOT copy the `.example` over; `sabokit up` writes the real file. Drop the placeholder from the scaffolded env tree.

## Out of scope (won't build)

- App-specific tooling (backrest UI passthrough, n8n workflow export, grafana dashboard import). Operators reach app UIs via `sabokit ssh` + `sabokit logs`.
- Replacing terraform or ansible. sabokit-cli sequences and validates; it does not reimplement the declarative engines.
- Flattening the two-pass Authentik bootstrap. Deferred to the federated-commons v4.0 TF→Blueprints migration.
- Replacing Scaleway-console-only steps (project creation, IAM key minting). Those stay manual, once per env.

## Versioning policy (as of v2026.05.0)

`vYYYY.MM.PATCH` calver. Patch resets on month rollover.

- First release in a month: `v2026.05.0`, `v2026.06.0`.
- Patches within a month bump the last component: `v2026.05.0`, `v2026.05.1`, …
- Pre-2026.05 releases on the `v0.1.x` line are frozen historical artifacts. Do not retag.
- Major shape changes (breaking API, runner-image rev that breaks call sites) are flagged in CHANGELOG entries, not in the version number — calver doesn't encode breaking-vs-additive.
