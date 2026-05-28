#!/usr/bin/env bash
# Build sabokit from the working tree and install it — for testing your own
# changes. Builds exactly what's on disk, uncommitted edits and all: a plain
# `go build` (debug symbols and real paths kept, unlike the release artifact),
# no clone, no fetch. For a prebuilt release binary instead, see install.sh.

set -eu

INSTALL_DIR="${SABOKIT_INSTALL_DIR:-}"
MODULE="github.com/sheyaln/sabokit-cli"

die() { printf 'error: %s\n' "$*" >&2; exit 1; }
info() { printf '%s\n' "$*"; }
warn() { printf 'warning: %s\n' "$*" >&2; }
have() { command -v "$1" >/dev/null 2>&1; }

# resolve_install_dir prefers /usr/local/bin if writable without sudo,
# falls back to $HOME/.local/bin (created if missing). caller can override
# with SABOKIT_INSTALL_DIR.
resolve_install_dir() {
  if [ -n "$INSTALL_DIR" ]; then
    mkdir -p "$INSTALL_DIR"
    printf '%s' "$INSTALL_DIR"
    return
  fi
  if [ -w /usr/local/bin ]; then
    printf '%s' /usr/local/bin
    return
  fi
  mkdir -p "$HOME/.local/bin"
  printf '%s' "$HOME/.local/bin"
}

# go_ge HAVE NEED — 0 if HAVE >= NEED under version sort.
go_ge() { [ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]; }

have go || die "go is not installed — needed to build from source (install.sh installs a prebuilt binary instead)"

# The source tree is wherever this script lives: repo root, beside go.mod.
src=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
{ [ -f "$src/go.mod" ] && [ -f "$src/cmd/sabokit/main.go" ] \
  && grep -q "^module ${MODULE}\$" "$src/go.mod"; } \
  || die "not a sabokit-cli checkout: expected go.mod + cmd/sabokit beside this script"

# Soft Go-version check. With GOTOOLCHAIN=auto (the default) an older toolchain
# fetches the version go.mod asks for, so this warns rather than fails.
need_go=$(awk '$1=="go"{print $2; exit}' "${src}/go.mod" 2>/dev/null || true)
have_go=$(go version | awk '{print $3}' | sed 's/^go//')
if [ -n "$need_go" ] && ! go_ge "$have_go" "$need_go"; then
  warn "go ${have_go} is older than go.mod's go ${need_go}; relying on GOTOOLCHAIN to fetch it (needs network, or install go >= ${need_go})"
fi

# Stamp version.CLI from git-describe so `sabokit version` shows a working-tree
# build (eg. 0.1.0-3-gabc123-dirty), telling it apart from a release. Override
# with SABOKIT_CLI_VERSION. Everything else (the supported-blueprint range) is
# left at its source default — this builds the code as written. Skip the stamp
# entirely if there's no git/describe, leaving the package default.
CLI_VERSION="${SABOKIT_CLI_VERSION:-}"
if [ -z "$CLI_VERSION" ] && have git && git -C "$src" rev-parse --git-dir >/dev/null 2>&1; then
  CLI_VERSION=$(git -C "$src" describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || true)
fi

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
bin="${tmpdir}/sabokit"

build_args=(-o "$bin")
[ -n "$CLI_VERSION" ] && build_args+=(-ldflags "-X ${MODULE}/internal/version.CLI=${CLI_VERSION}")
build_args+=(./cmd/sabokit)

info "building sabokit ${CLI_VERSION:-(package default)} from ${src}"
( cd "$src" && go build "${build_args[@]}" ) || die "build failed"

dir=$(resolve_install_dir)
target="${dir}/sabokit"
install -m 0755 "$bin" "$target"
info "installed sabokit to ${target}"

case ":${PATH}:" in
  *":${dir}:"*) ;;
  *) warn "${dir} is not in PATH — add it to your shell profile" ;;
esac

"$target" version
