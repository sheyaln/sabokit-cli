# sabokit-cli

cli for deploying and operating sabokit stacks. the consumer-template layer scripts are the runbook; sabokit runs them inside the `sabokit-runner` image (terraform + ansible + scw baked in) and adds preflight checks, confirmation gates, and scaffolding on top. you install none of the tooling locally.

## install

```bash
curl -fsSL https://raw.githubusercontent.com/sheyaln/sabokit-cli/master/install.sh | bash
```

binary lands in `/usr/local/bin` if writable, else `$HOME/.local/bin`. override with `SABOKIT_INSTALL_DIR=...`. pin a version with `SABOKIT_VERSION=v0.1.0` (semver: `vX.Y.Z`).

### from source

for testing your own changes. builds the working tree as-is — a plain `go build`, uncommitted edits and all — and installs it. needs `go`; run it from a checkout:

```bash
./install-from-source.sh
```

same install-dir resolution as above (`SABOKIT_INSTALL_DIR` to override). `sabokit version` reports a `git describe` of the tree (eg. `0.1.0-dirty`) so you can tell a working-tree build from a release; override with `SABOKIT_CLI_VERSION=...`.

## quickstart

```bash
export SCW_ACCESS_KEY=... SCW_SECRET_KEY=... SCW_DEFAULT_PROJECT_ID=...

sabokit init my-stack
# scaffolds project + env(s) + state buckets end-to-end. prompts for
# base_domain, scaleway project UUID, org_slug, org_name, infra_email,
# ssh user/key, first env (default prod), optional staging.

cd my-stack

# review the per-layer YAML — apps, tiers, hosts, watchers
$EDITOR environments/prod/application.yml

sabokit up   # preflight + all four layers (infra → identity →
             # operations → application), end-to-end, one command

# now sabokit takes over (env is auto from default_env):
sabokit deploy --apps espocrm --check     # ansible-only redeploy, dry run
sabokit deploy --apps espocrm             # ansible-only redeploy
sabokit up --layers application           # tf + ansible for app changes
sabokit status                            # enabled apps + docker ps
sabokit secrets list --tag authentik      # works independently
```

run `sabokit quickstart` for the full walkthrough with troubleshooting.

## requires

- docker (daemon must be running for `up`/`deploy`/`down`/`destroy`/`status`/`refresh`/`secrets`)
- ssh (for `ssh`/`logs` and as ansible's transport)

## commands

| command | what |
| --- | --- |
| `sabokit init <name> [--env X --base-domain X --region X --ssh-user/-key X --non-interactive]` | clone consumer-template at pinned tag, scaffold `environments/<env>/` (env.yml + per-layer backend.hcl), write `.sabokit/config.yml`, create state buckets |
| `sabokit config init [--force]` / `sabokit config show` | interactively (re)generate `.sabokit/config.yml` in cwd, or print the loaded config + path |
| `sabokit up [--layers X,Y --skip-preflight --no-confirm]` | end-to-end bring-up: preflight (creds, ssh key, DNS zone, backends, bucket) + one confirmation, then the four layer scripts in order inside the runner |
| `sabokit deploy [--apps X --servers Y --all --rotate-secrets --check]` | scripts/deploy.sh — ansible-only redeploy (no terraform) with the selected tags |
| `sabokit refresh` | scripts/refresh.sh — regenerate `inventory.ini` + `.enabled_apps.json` from terraform state |
| `sabokit down --apps X [--servers Y]` | ansible-playbook down.yml — stop containers, leave cloud resources |
| `sabokit destroy {--layer infra\|identity\|operations\|application \| --all} [-y]` | scripts/destroy-layer.sh or scripts/down.sh — terraform destroy per layer or everything in reverse dependency order |
| `sabokit status [--apps X] [--servers Y]` | enabled apps (from `.enabled_apps.json`) + `docker ps` across hosts |
| `sabokit ssh <host>` | passthrough to `ssh <user>@<host>` |
| `sabokit logs <app> [--servers H] [--group G] [--container C] [--tail N] [-f]` | `docker logs` over ssh — host resolved from env's `inventory.ini` `[apps]` group by default, override with `--group` (eg. `identity`) or `--servers` |
| `sabokit secrets list/get/versions/create/rotate/delete` | name-first scaleway secret management — collapses uuid lookups + base64 decoding |
| `sabokit apps list [--enabled]` | catalog (NAME, CATEGORY, DESCRIPTION) by default; `--enabled` shows env-resolved enabled apps with URLs from `.enabled_apps.json` |
| `sabokit apps add <name>` / `sabokit apps remove <name>` | edit `environments/<env>/application.yml` — uncomment or flip `enabled:` for the named app, validated against the catalog |
| `sabokit env add <name> --from <env>` | scaffold a new env by carbon-copying an existing one (per-layer backends regenerated, env.yml reset, state bucket created) |
| `sabokit version` | binary version + supported blueprint range + every env's pin |

global flags:
- `--image <repo:tag>` (default: `ghcr.io/sheyaln/sabokit-runner` at the env's pinned sabokit version) — runner image for ansible; env `SABOKIT_IMAGE`
- `--scw-image <repo:tag>` (default `scaleway/cli:2.56`) — official scaleway cli image used for `secrets *`; env `SABOKIT_SCW_IMAGE`
- `--platform <p>` — docker `--platform` override; defaults to the host's native arch (all images are multi-arch). Set eg. `linux/amd64` to force emulation; env `SABOKIT_PLATFORM`
- `--pull <policy>` — runner image docker `--pull` policy (`always`/`missing`/`never`); default `always` so a moved tag is refreshed (the runner rides the moving pre-launch tag). Set `missing`/`never` for offline or pre-pulled CI; env `SABOKIT_PULL`
- `--env <name>` — environment name under `environments/<env>/`; overrides `.sabokit/config.yml`'s `default_env`; env `SABOKIT_ENV`
- `-v/--verbose`

most runner-backed commands accept `--print` to print the `docker run` invocation without executing it.

## project layout

a sabokit project is any directory containing `.sabokit/config.yml`. sabokit walks up from cwd to find it.

```yaml
# .sabokit/config.yml
project: dciww-staging
base_domain: example.com
default_env: prod          # env that sabokit operates against by default
scaleway:
  region: fr-par
  zone: fr-par-1
ssh:
  user: root
  key: ~/.ssh/id_ed25519
inventory: inventory.ini        # default; resolved inside the env dir
apps_manifest: apps-manifest.yaml  # default; project root
```

with `default_env: prod`, sabokit mounts the whole consumer repo at `/workspace/consumer` in the runner image and runs the layer scripts with the repo as cwd. the env's committed YAML (`env.yml` + per-layer files), the per-layer terraform roots, and the derived `inventory.ini` / `.enabled_apps.json` all live under `environments/prod/` (consumer-template's shape). override per-call with `--env <name>` or `SABOKIT_ENV=<name>`.

the layer scripts themselves resolve consumer-first: a vendored `scripts/` in your repo wins; otherwise sabokit runs the copy baked into the runner image at exactly the blueprint tag your env pins.

## status

beta. shipped surface: `init`, `config init|show`, `up`, `deploy`, `refresh`, `down`, `status`, `destroy`, `apps list|add|remove`, `env add`, `ssh`, `logs`, `secrets *`, `quickstart`, `version`. roadmap + status table in [FEATURES.md](FEATURES.md). versions are semver `vX.Y.Z`; the CLI supports a range of blueprint versions and each env pins its own via the per-layer terraform `?ref=` (see `sabokit version`).

execution models per command:
- **docker (sabokit-runner image)**: `up`, `deploy`, `refresh`, `destroy`, `down`, `status` (container-state section). `ghcr.io/sheyaln/sabokit-runner` tagged at the env's pinned sabokit version; terraform + ansible + scw + the platform tree + consumer-template baked in. multi-arch (`linux/amd64` + `linux/arm64`); the CLI selects the host's arch automatically.
- **docker (scaleway/cli image)**: `secrets *`, `up` preflight (project probe, ssh-key upload, DNS zone check, state bucket). Decoupled from the runner image.
- **local**: `init` (git clone + copy of consumer-template into your project dir), `ssh` (`ssh user@host` passthrough), `logs` (ssh + remote docker), `apps add/remove` (application.yml editing), `env add` (tree copy + backend regen).

the only host requirements are `docker` + `ssh` (+ `git` for `sabokit init`). no terraform, ansible, scw, jq, python3, awk, nc, ssh-keygen on the host. nothing else gets cloned or cached locally beyond what `init` writes into your project dir.
