# sabokit-cli features plan

Operator-facing roadmap. Lives in-repo so the plan moves with the code.

## Mandate

sabokit-cli is the conductor for the deploy lifecycle and the consumer-template scaffolding. The `consumer-template/scripts/` layer scripts are the runbook and the single source of truth — the CLI shells out to them inside the runner image (consumer's vendored copy first, baked copy at the pinned tag otherwise) and adds preflight checks, confirmation gates, and scaffolding. Operators never `chmod +x` shell scripts, hand-create state buckets, or `cp -r` scaffolding dirs. Terraform stays declarative. Ansible stays declarative.

## Host requirements

`docker` + `ssh` + `git` on the host. Nothing else. terraform/ansible/scw/jq/python come from images sabokit invokes (`ghcr.io/sheyaln/sabokit-runner`, `scaleway/cli:2.56`).

## Status (as of the v0.2.0 four-layer rework)

| Area | Shipped | Open |
| --- | --- | --- |
| `sabokit up` four-layer end-to-end | preflight (env.yml keys, distinct project ids, SCW probe, IAM ssh-key upload, DNS-zone delegation, per-layer backend.hcl generation, state-bucket create) + one confirm gate + `scripts/up.sh` (or `--layers` subset) in the runner | — |
| Layer-script execution model | consumer `scripts/` wins, else baked `/opt/sabokit/consumer-template/scripts/` at the env's pinned tag; repo mounted at `/workspace/consumer`; `/workspace/sabokit` symlink resolves the sibling import | — |
| `sabokit deploy/refresh/destroy/down` | deploy → `deploy.sh` (ansible-only); refresh → `refresh.sh`; destroy → `destroy-layer.sh`/`down.sh`; down → `down.yml` (compose stop) | — |
| `sabokit init` | four-layer scaffold: common.yml org identity, per-env env.yml, per-layer backend.hcl ×4, state buckets | interactive review of per-layer YAML (apps, tiers) |
| `sabokit env add --from` | four-layer carbon-copy: per-layer backend regen, bucket-name suffix swap, env.yml seeding from legacy env-values.yml block or placeholders | `env list`, `env switch`, non-`--from` variant |
| `sabokit apps add/remove` | edits `environments/<env>/application.yml` textually (uncomment / flip `enabled:`), catalog-validated | — |
| Compat gate | per-env pin from the unique `?ref=` across the four layer roots; CLI declares supported range (0.2 line) | — |

## Ordered packs

Each row is one branch, one commit-set, one tag. No PR ceremony; merge to master and tag.

1. **`sabokit env list/switch`** — `env list` enumerates `environments/`; `env switch <name>` rewrites `.sabokit/config.yml` `default_env`. The non-`--from` interactive variant of `env add` rides along (reuses the init prompt + bucket flow).
2. **`sabokit bump <tag>`** — rewrite the `?ref=` pin in every layer root's stack.tf, prompt to run any `moved{}` migrations upstream publishes for the new tag.
3. **`sabokit upgrade <tag>`** — `bump` + per-layer apply chain. Pre-apply check refuses state-mutation if any legacy address exists that isn't mapped by a `moved{}` block.
4. **`sabokit doctor`** — state bucket reachable per layer; terraform state present + non-corrupt; every compute host ssh-reachable; remote docker daemon up; traefik routing healthy; certs not within expiry window; most-recent backrest run completed; loki receiving from monitoring agent.
5. **`sabokit import-base-image <tag>`** — wrap `consumer-template/scripts/import-base-image.sh` (download qcow2, object-storage upload, snapshot import, image registration) behind one verb.
6. **`sabokit cost-estimate`** — monthly burn from the hosts.yml/env.yml shape × scaleway pricing API.
7. **`sabokit version-check`** — github tags API ping, flag if consumer is behind.

## Scaffolding non-negotiables

- **`.tfvars` is for secrets only.** Operator-facing config (apps, hostnames, tiers, watchers) lives in the committed per-layer YAML (`env.yml`, `application.yml`, ...). The `env add --from <env>` carbon-copy flow MUST NOT copy `.tfvars` across envs — each env's secrets are its own. `.gitignore` excludes `*.tfvars` and keeps `*.tfvars.example` tracked.
- **`inventory.ini` is always generated.** The real inventory comes from `terraform output -json compute` on every layer-script run (`scripts/refresh.sh` regenerates it on demand). Never scaffold or hand-edit one.
- **`terraform.tfvars` is not generated at init.** Consumers don't need one for a normal deploy; the secret path is scaleway secret manager via the layers' data sources, and the Authentik admin token is fetched by `lib.sh` at run time.

## Hardening backlog (non-blocking)

1. **Bucket-exists-but-different-ownership guard** — when `BucketExists` says yes, re-fetch and verify it's in the current `SCW_DEFAULT_PROJECT_ID`. Detects S3-namespace collisions (rare but irreversibly confusing when they happen). ~15 lines.
2. **Init-time SSH-key IAM registration + DNS-zone-delegation check** — promote both from `up` preflight to also run during `init`, so a 30-second `init` can't feel successful when the first `up` is going to fail anyway. Costs the operator nothing today since `up` catches them.

## Out of scope (won't build)

- App-specific tooling (backrest UI passthrough, n8n workflow export, grafana dashboard import). Operators reach app UIs via `sabokit ssh` + `sabokit logs`.
- Replacing terraform or ansible. sabokit-cli sequences and validates; it does not reimplement the declarative engines.
- Flattening Authentik's deploy-then-configure rhythm. The identity layer scripts own it; a future TF→Blueprints migration may collapse it upstream.
- Replacing Scaleway-console-only steps (project creation, IAM key minting). Those stay manual, once per env.

## Versioning policy

`vX.Y.Z` semver. **The CLI does not choose a blueprint version — each environment does**, via the unique `?ref=` across its four layer roots' `*.tf` (`environments/<env>/{infra,identity,operations,application}/stack.tf`). The CLI reads that pin, runs the matching `sabokit-runner` image, and verifies it can drive it.

- **The CLI declares a supported blueprint range**, not a single pin: `internal/version.SupportedBlueprintMin`–`SupportedBlueprintMax` (major.minor lines). `Min` is a source constant — bump it when dropping support for an old line; it sits at `0.2` because the four-layer layout is the floor. `Max` is injected at build = the CLI tag's own major.minor.
- **The runner image follows the env's pin.** The runner tag is resolved from the env's pinned version, so the script/ansible half always matches the terraform half — and the baked layer scripts match too. `--image` / `SABOKIT_IMAGE` override.
- **Compatibility is gated at action time.** `up`/`deploy`/`refresh`/`down`/`destroy` refuse to run when the env's pinned version falls outside the supported range, with remediation (upgrade the CLI, or re-pin the env). Override: `--skip-version-check` (unsafe — mismatched TF/Ansible can corrupt state). `sabokit version` prints the CLI version, the supported range, and every env's pin with an `[ok]`/`[UNSUPPORTED]` mark.
- **Stable vs beta is just the pinned ref.** Pin `?ref=v0.5.0-beta1` and you get the beta terraform *and* the beta runner image, reproducibly, per-env. No CLI channel flag.
- **dev / source builds** report `-dev` (or `git describe`) and leave the supported range at its source default.
- **Breaking changes** bump major/minor per semver and are flagged in CHANGELOG entries.
