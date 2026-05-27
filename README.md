# sabokit-cli

cli for deploying and operating federated-commons stacks. shells out to the `sabokit-runner` image for terraform + ansible; you don't install either locally.

## install

```bash
curl -fsSL https://raw.githubusercontent.com/sheyaln/sabokit-cli/master/install.sh | bash
```

binary lands in `/usr/local/bin` if writable, else `$HOME/.local/bin`. override with `SABOKIT_INSTALL_DIR=...`. pin a version with `SABOKIT_VERSION=v0.1.0`.

## quickstart

```bash
sabokit init my-stack --env prod --base-domain example.com   # project + env in one shot

cd my-stack

export SCW_ACCESS_KEY=... SCW_SECRET_KEY=... SCW_DEFAULT_PROJECT_ID=...
# arm64 hosts: the runner image is amd64-only
export SABOKIT_PLATFORM=linux/amd64

# fill in the env config (consumer-template's own flow):
cd environments/prod
cp config.tf.example   config.tf   && $EDITOR config.tf
cp backend.hcl.example backend.hcl && $EDITOR backend.hcl
cp inventory.ini.example inventory.ini
./preflight.sh && ./up.sh && ./configure.sh
cd ../..

# now sabokit takes over (env is auto from default_env):
sabokit deploy --apps espocrm --check     # apps.yml --check
sabokit deploy --apps espocrm             # apps.yml for real
sabokit status                            # tf outputs + docker ps
sabokit secrets list --tag authentik      # works independently
```

run `sabokit quickstart` for the full walkthrough with troubleshooting.

## requires

- docker (daemon must be running for `deploy`/`down`/`status`/`up`/`secrets`)
- ssh (for `ssh`/`logs` and as ansible's transport)

## commands

| command | what |
| --- | --- |
| `sabokit init <name> [--env X --base-domain X --region X --ssh-user/-key X --non-interactive]` | clone consumer-template at pinned tag, optionally bootstrap `environments/<env>/`, write `.sabokit/config.yml` |
| `sabokit up [--skip-preflight --skip-up --skip-configure --template-tag T]` | chain `preflight.sh && up.sh && configure.sh` locally in the env dir; auto-clones `FED_COMMONS_DIR` to `.sabokit/sabokit-repo/` |
| `sabokit deploy [--apps X --servers Y --base --rotate-secrets --check --overlay F]` | ansible-playbook apps.yml (or site.yml with --base) against `environments/<env>/` |
| `sabokit down --apps X [--servers Y]` | ansible-playbook down.yml — stop containers, leave cloud resources |
| `sabokit destroy {--apps X[,Y] \| --layer base\|identity\|apps \| --all} [-y]` | local `terraform destroy` in the env dir with the right `-target=` |
| `sabokit status [--apps X] [--servers Y]` | terraform outputs per layer + `docker ps` across hosts |
| `sabokit ssh <host>` | passthrough to `ssh <user>@<host>` |
| `sabokit logs <app> [--servers H] [--group G] [--container C] [--tail N] [-f]` | `docker logs` over ssh — host resolved from env's `inventory.ini` `[apps]` group by default, override with `--group` (eg. `identity`) or `--servers` |
| `sabokit secrets list/get/versions/create/rotate/delete` | name-first scaleway secret management — collapses uuid lookups + base64 decoding |
| `sabokit apps list [--enabled]` | catalog (NAME, CATEGORY, DESCRIPTION) by default; `--enabled` shows env-resolved enabled apps with URLs from `.ansible-vars.json` |
| `sabokit apps add <name>` / `sabokit apps remove <name>` | edit `environments/<env>/config.tf` — uncomment or flip `enabled` for the named app, validated against the catalog |
| `sabokit version` | binary version + default runner image |

global flags:
- `--image <repo:tag>` (default `ghcr.io/sheyaln/sabokit-runner:v3.3.1`) — runner image for ansible; env `SABOKIT_IMAGE`
- `--scw-image <repo:tag>` (default `scaleway/cli:2.56`) — official scaleway cli image used for `secrets *`; env `SABOKIT_SCW_IMAGE`
- `--platform <p>` — docker `--platform` override (eg. `linux/amd64` on arm64 hosts when an image is amd64-only); env `SABOKIT_PLATFORM`
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

with `default_env: prod`, sabokit mounts `environments/prod/` as `/workspace` in the runner image. `inventory.ini`, `config.tf`, `.tf-output.json`, and `.ansible-vars.json` all live there (consumer-template's shape). override per-call with `--env <name>` or `SABOKIT_ENV=<name>`.

## status

beta. v0.1.6 is feature-complete for the v0.1.x line: `init`, `up`, `deploy`, `down`, `status`, `destroy`, `apps list/add/remove`, `ssh`, `logs`, `secrets *`, `quickstart`, `version`.

execution models per command:
- **docker (sabokit-runner image)**: `deploy`, `down`, `status` (container-state section). Default `ghcr.io/sheyaln/sabokit-runner:v3.3.1`; playbooks at `/opt/sabokit/platform/ansible/`. Image is amd64-only — set `SABOKIT_PLATFORM=linux/amd64` on arm64 hosts.
- **docker (scaleway/cli image)**: `secrets *`. Decoupled from the runner image.
- **local shell**: `up` (chains the consumer-template scripts), `destroy` (terraform destroy in the env dir), `apps add/remove` (config.tf editing), `init` (git clone + copy), `ssh`, `logs` (ssh + remote docker).

`up` requires local `terraform`, `ansible`, `scw`, `jq`, `python3`, `awk`, `nc`, `ssh`, `ssh-keygen` — the same deps the underlying scripts have always needed.
