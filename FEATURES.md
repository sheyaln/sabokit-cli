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
| State bucket creation | per-env idempotent create via `scaleway/cli`; versioning enabled; `acl=private`; name-length (≤63) precheck | block-public-access (scw doesn't expose a flag; `acl=private` is canonical); bucket-exists-but-different-ownership guard (currently skips silently on name collision) |
| Preflight (phase 0 of `up`) | config keys + SCW creds + IAM ssh-key upload + DNS-zone-delegation check; gateway DNS propagation wait runs between bootstrap apply and LE-cert wait | — |
| Init-time validation echo | env-var presence check for `SCW_ACCESS_KEY`/`SCW_SECRET_KEY`; bucket-name length precheck | SCW project probe (`account project get`) before bucket create; SSH-key IAM registration at init (currently deferred to `up`); DNS zone delegation check at init (currently deferred to `up`) |
| `sabokit up` end-to-end | preflight + plan+confirm+apply (bootstrap) + refresh + ansible bootstrap + DNS-propagation wait + LE-cert wait + Authentik index wait + plan+confirm+apply (configure) + outpost import. Plan-confirm gate defaults yes for non-prod, no for prod; `--no-confirm` bypasses both gates | — |
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
2. ~~**Preflight DNS + plan-confirm gate**~~ — shipped v2026.05.3.
3. **`sabokit env add/list/switch`** — `env add <name> [--from <existing-env>]` adds an additional env. Without `--from`, reuses the init prompt + bucket flow. With `--from`, carbon-copies committable files from `environments/<existing-env>/` into `environments/<name>/` (config.tf, README.md, any moved.tf, providers.tf, secrets.tf, variables.tf, main.tf, *.example files) and **skips everything gitignored** (`terraform.tfvars`, `backend.hcl`, `inventory.ini`, `.terraform/`, `.terraform.lock.hcl`). Backend.hcl is regenerated for the new env (bucket name swap). Operator edits config.tf afterwards for env-specific values (project_id, domains, environment string). `env list` enumerates `environments/`. `env switch <name>` rewrites `.sabokit/config.yml` `default_env`.
   - **partial ship**: `env add <name> --from <existing-env>` landed standalone to unblock dciww-commons staging rebuilds. The `environment = "..."` HCL assignment in config.tf is auto-rewritten; bucket created via the same scw helper init uses. `env list`, `env switch`, and the non-`--from` interactive variant of `env add` still open.
4. **`sabokit bump <tag>`** — rewrite every `?ref=…` pin under the consumer's TF, optionally bump a sibling sabokit submodule, prompt to run any `moved{}` migrations published in upstream's `consumer-template/modules/stack/migrations.tf` for the new tag.
5. **`sabokit upgrade <tag>`** — `bump` + `deploy` chain. Pre-apply check refuses state-mutation if any legacy address exists that isn't mapped by a `moved{}` block in the consumer's migrations.tf (backrest-bucket foot-gun).
6. **`sabokit doctor`** — state bucket reachable; terraform state file present + non-corrupt; every compute host ssh-reachable; remote docker daemon up; traefik routing healthy; certs not within expiry window; most-recent backrest run completed; loki receiving from monitoring agent.
7. **`sabokit import-base-image <tag>`** — pull `fc-base-<tag>.qcow2` from github release, upload via scaleway object storage + block snapshot + image registration, print resulting `image_id`, offer to inline-edit the relevant `compute_hosts.*.image` key in `config.tf`.
8. **`sabokit cost-estimate`** — monthly burn from `compute_hosts` shape × scaleway pricing API.
9. **`sabokit version-check`** — github tags API ping, flag if consumer is behind.

## Scaffolding non-negotiables

Two rules the upstream operator has surfaced (both **honored as of v2026.05.2**):

- **`.tfvars` is for secrets only.** Operator-facing config (apps, hostnames, image tags, host-services knobs) lives in `config.tf` as `locals.config` + `module "stack"`. The init setup wizard MAY write a `.tfvars` for plaintext-secret-at-apply-time. The `env add --from <env>` carbon-copy flow MUST NOT copy `.tfvars` across envs — each env's secrets are its own. `.gitignore` excludes `*.tfvars` and keeps `*.tfvars.example` tracked.
- **`inventory.ini.example` is misleading.** The real `inventory.ini` is generated from `terraform output -json compute_hosts` on every `sabokit up`/`deploy`. `scaffoldEnv` strips the placeholder alongside the legacy bash orchestration scripts (`preflight.sh`, `up.sh`, `configure.sh`, `_lib.sh`) — sabokit-cli replaced them.
- **`terraform.tfvars` is not generated at init.** `init` writes none; consumers don't need one for a normal deploy. The wizard reserves the right to write a `.tfvars` for a future plaintext-secret-at-apply-time variable, but no such variable exists in upstream today. Current secret path is scaleway secret manager via `data "scaleway_secret_version"` blocks in `secrets.tf`.

## Hardening backlog (non-blocking for first E2E)

These are gaps surfaced by the v2026.05.3 audit. None block a fresh consumer from doing `sabokit init` → `sabokit up` end-to-end today, because `up`'s preflight covers the same checks before any terraform apply runs. Listed by fail-loud-earliness payoff:

1. **Init-time SCW project probe** — call `scw account project get project-id=<id>` once before the bucket-create loop. Catches wrong/stale API keys with a clean error instead of an opaque `scw object bucket create` failure. ~10 lines.
2. **Bucket-exists-but-different-ownership guard** — when `BucketExists` says yes, re-fetch and verify it's in the current `SCW_DEFAULT_PROJECT_ID`. Detects S3-namespace collisions (rare but irreversibly confusing when they happen). ~15 lines.
3. **Init-time SSH-key IAM registration** — promote `EnsureSSHKey` from `up` preflight to also run during `init`. Means the operator finds out about a missing public key when scaffolding, not on first `up`.
4. **Init-time DNS-zone-delegation check** — promote `ensureDNSZoneDelegated` from `up` preflight to also run during `init`. Same shape as #3 — earlier, louder failure.

Items 1–2 are foundation-tail and ship as a single patch (v2026.05.4) before pack 3. Items 3–4 are creature comfort: they prevent a 30-second `init` from feeling successful when the first `up` is going to fail anyway, but cost the operator nothing today since `up` catches them. Bundle into pack 3 (`env add` interactive) or defer.

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
