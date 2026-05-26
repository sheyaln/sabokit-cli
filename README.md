# sabokit-cli

cli for deploying and operating federated-commons stacks. shells out to the `sabokit-runner` image for terraform + ansible; you don't install either locally.

## install

```bash
curl -fsSL https://raw.githubusercontent.com/sheyaln/sabokit-cli/master/install.sh | bash
```

binary lands in `/usr/local/bin` if writable, else `$HOME/.local/bin`. override with `SABOKIT_INSTALL_DIR=...`. pin a version with `SABOKIT_VERSION=v0.1.0`.

## quickstart

```bash
sabokit init my-stack --base-domain example.com   # scaffolds from consumer-template

cd my-stack

export SCW_ACCESS_KEY=... SCW_SECRET_KEY=... SCW_DEFAULT_PROJECT_ID=...
# default runner image v3.0.0 is not yet published — point at the older one:
export SABOKIT_IMAGE=ghcr.io/sheyaln/federated-commons-runner:v2.17.0
# arm64 hosts also need:
export SABOKIT_PLATFORM=linux/amd64

sabokit secrets list                              # works independently
sabokit apps list

# per-env setup (consumer-template's own flow):
cp -r environments/_template environments/prod
cd environments/prod
cp terraform.tfvars.example terraform.tfvars && $EDITOR terraform.tfvars
cp backend.hcl.example backend.hcl && $EDITOR backend.hcl
cp inventory.ini.example inventory.ini
./preflight.sh && ./up.sh && ./configure.sh
```

run `sabokit quickstart` for the full walkthrough with troubleshooting and the architectural caveats.

## requires

- docker (daemon must be running for `deploy`/`down`/`status`/`up`/`secrets`)
- ssh (for `ssh`/`logs` and as ansible's transport)

## commands

| command | what |
| --- | --- |
| `sabokit init <name> [--base-domain X --region X --ssh-user/-key X --non-interactive]` | clone consumer-template at pinned tag, write `.sabokit/config.yml` |
| `sabokit up [--apps X]` | terraform base+identity+apps then ansible deploy — not yet implemented |
| `sabokit deploy [--apps X --servers Y --base/--no-base --rotate-secrets --check --overlay F]` | ansible-playbook deploy.yml inside runner image |
| `sabokit down --apps X [--servers Y]` | ansible-playbook down.yml — stop containers, leave cloud resources |
| `sabokit destroy` | not yet implemented — destroy is manual via terraform in v0.1.0 |
| `sabokit status [--apps X] [--servers Y]` | terraform outputs per layer + `docker ps` across hosts |
| `sabokit ssh <host>` | passthrough to `ssh <user>@<host>` |
| `sabokit logs <app> [--servers H] [--container C] [--tail N] [-f]` | `docker logs` over ssh |
| `sabokit secrets list/get/versions/create/rotate/delete` | name-first scaleway secret management — collapses uuid lookups + base64 decoding |
| `sabokit apps list [--enabled]` | tabular list of apps in apps-manifest.yaml (NAME, ENABLED, HOST) |
| `sabokit apps add/remove` | edit apps-manifest.yaml + regenerate apps.tf — not yet implemented |
| `sabokit version` | binary version + default runner image |

global flags:
- `--image <repo:tag>` (default `ghcr.io/sheyaln/sabokit-runner:v3.0.0`) — runner image for ansible/terraform; env `SABOKIT_IMAGE`
- `--scw-image <repo:tag>` (default `scaleway/cli:2.56`) — official scaleway cli image used for `secrets *`; env `SABOKIT_SCW_IMAGE`
- `--platform <p>` — docker `--platform` override (eg. `linux/amd64` on arm64 hosts when an image is amd64-only); env `SABOKIT_PLATFORM`
- `-v/--verbose`

most runner-backed commands accept `--print` to print the `docker run` invocation without executing it.

## project layout

a sabokit project is any directory containing `.sabokit/config.yml`. sabokit walks up from cwd to find it.

```yaml
# .sabokit/config.yml
project: dciww-staging
scaleway:
  region: fr-par
  zone: fr-par-1
ssh:
  user: root
  key: ~/.ssh/id_ed25519
inventory: inventory.ini        # default
apps_manifest: apps-manifest.yaml  # default
```

`apps-manifest.yaml` lives alongside and is parsed for the enabled-apps list and per-app host mapping:

```yaml
apps:
  espocrm:
    enabled: true
    host: app01
  authentik:
    enabled: true
    host: app02
```

## status

beta. v0.1.3 ships `init`/`deploy`/`down`/`status`/`ssh`/`logs`/`apps list`/`secrets *`/`version`. `up`/`destroy`/`apps add|remove` are stubs. requires the `sabokit-runner:v3.0.0` image for the ansible playbooks (against the older `federated-commons-runner:v2.17.0`, `deploy` works but `down` and parts of `status` won't). `secrets *` uses the official `scaleway/cli` image and is independent of the runner image.

known architectural gap: `sabokit-cli`'s project model assumes flat root (one `inventory.ini`, `apps-manifest.yaml` at root); consumer-template uses per-env dirs (`environments/<env>/inventory.ini`). `init` produces consumer-template's layout, but `deploy`/`status` won't fully match until sabokit-cli grows env-aware paths. Tracked.
