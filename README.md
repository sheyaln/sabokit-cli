# sabokit-cli

cli for deploying and operating federated-commons stacks. shells out to the `sabokit-runner` image for terraform + ansible; you don't install either locally.

## install

```bash
curl -fsSL https://raw.githubusercontent.com/sheyaln/sabokit-cli/master/install.sh | bash
```

binary lands in `/usr/local/bin` if writable, else `$HOME/.local/bin`. override with `SABOKIT_INSTALL_DIR=...`. pin a version with `SABOKIT_VERSION=v0.1.0`.

## requires

- docker (daemon must be running for `deploy`/`down`/`status`/`up`/`secrets`)
- ssh (for `ssh`/`logs` and as ansible's transport)

## commands

| command | what |
| --- | --- |
| `sabokit init <name>` | scaffold a consumer project — not yet implemented |
| `sabokit up [--apps X]` | terraform base+identity+apps then ansible deploy — not yet implemented |
| `sabokit deploy [--apps X --servers Y --base/--no-base --rotate-secrets --check --overlay F]` | ansible-playbook deploy.yml inside runner image |
| `sabokit down --apps X [--servers Y]` | ansible-playbook down.yml — stop containers, leave cloud resources |
| `sabokit destroy` | not yet implemented — destroy is manual via terraform in v0.1.0 |
| `sabokit status [--apps X] [--servers Y]` | terraform outputs per layer + `docker ps` across hosts |
| `sabokit ssh <host>` | passthrough to `ssh <user>@<host>` |
| `sabokit logs <app> [--servers H] [--container C] [--tail N] [-f]` | `docker logs` over ssh |
| `sabokit secrets create/rotate` | scaleway secret operations — not yet implemented |
| `sabokit apps list [--enabled]` | tabular list of apps in apps-manifest.yaml (NAME, ENABLED, HOST) |
| `sabokit apps add/remove` | edit apps-manifest.yaml + regenerate apps.tf — not yet implemented |
| `sabokit version` | binary version + default runner image |

global flags: `--image <repo:tag>` (default `ghcr.io/sheyaln/sabokit-runner:v3.0.0`), `-v/--verbose`.

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

beta. v0.1.0 ships `deploy`/`down`/`status`/`ssh`/`logs`/`version`. `init`/`up`/`destroy`/`secrets`/`apps` are stubs. requires the `sabokit-runner:v3.0.0` image for the ansible playbooks; against the older `federated-commons-runner:v2.17.0` image, `deploy` works but `down` and parts of `status` won't.
