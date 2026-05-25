#!/usr/bin/env bash
# wrap the entire script in a brace group so bash parses it completely
# before executing any statement. without this, macOS bash 3.2 reading from
# a pipe (curl | bash) can choke on function definitions partway through
# the input and emit phantom syntax errors. used by docker / rustup / etc.
{

set -eu

REPO="${SABOKIT_REPO:-sheyaln/sabokit-cli}"
VERSION="${SABOKIT_VERSION:-latest}"
INSTALL_DIR="${SABOKIT_INSTALL_DIR:-}"

die() { printf 'error: %s\n' "$*" >&2; exit 1; }
info() { printf '%s\n' "$*"; }

have() { command -v "$1" >/dev/null 2>&1; }

detect_os() {
  case "$(uname -s)" in
    Linux)  printf linux ;;
    Darwin) printf darwin ;;
    *) die "unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  printf amd64 ;;
    arm64|aarch64) printf arm64 ;;
    *) die "unsupported arch: $(uname -m)" ;;
  esac
}

# resolve_install_dir prefers /usr/local/bin if writable without sudo,
# falls back to $HOME/.local/bin (created if missing). caller can override
# with SABOKIT_INSTALL_DIR.
resolve_install_dir() {
  if [ -n "$INSTALL_DIR" ]; then
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

# download URL OUTFILE — uses curl if present, else wget. fails the script
# on any non-200 response or transport error.
download() {
  url=$1
  out=$2
  quiet=${3:-}
  if have curl; then
    if [ -n "$quiet" ]; then
      curl -fsSL --proto '=https' --tlsv1.2 --connect-timeout 15 --retry 2 -o "$out" "$url" \
        || die "download failed: $url"
    else
      curl -fL --progress-bar --proto '=https' --tlsv1.2 --connect-timeout 15 --retry 2 -o "$out" "$url" \
        || die "download failed: $url"
    fi
  elif have wget; then
    wget --https-only --timeout=15 --tries=3 -qO "$out" "$url" \
      || die "download failed: $url"
  else
    die "neither curl nor wget is installed"
  fi
}

# verify_sha BINFILE SHAFILE — checks the binary against the sha256 sidecar.
# falls back to a warning (not fatal) if no sha256 tool is available, so
# the install still works on minimal systems.
verify_sha() {
  bin=$1
  sha=$2
  expected=$(awk '{print $1}' "$sha")
  if have shasum; then
    actual=$(shasum -a 256 "$bin" | awk '{print $1}')
  elif have sha256sum; then
    actual=$(sha256sum "$bin" | awk '{print $1}')
  else
    printf 'warning: no sha256 tool found, skipping checksum verification\n' >&2
    return 0
  fi
  if [ "$expected" != "$actual" ]; then
    die "checksum mismatch — expected $expected, got $actual"
  fi
}

os=$(detect_os)
arch=$(detect_arch)
asset="sabokit-${os}-${arch}"

if [ "$VERSION" = "latest" ]; then
  base="https://github.com/${REPO}/releases/latest/download"
else
  base="https://github.com/${REPO}/releases/download/${VERSION}"
fi

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

bin="${tmpdir}/${asset}"
sha="${tmpdir}/${asset}.sha256"

info "downloading ${base}/${asset}"
download "${base}/${asset}" "$bin"

download "${base}/${asset}.sha256" "$sha" quiet
info "verifying checksum"
verify_sha "$bin" "$sha"

dir=$(resolve_install_dir)
target="${dir}/sabokit"
install -m 0755 "$bin" "$target"
info "installed sabokit to ${target}"

case ":${PATH}:" in
  *":${dir}:"*) ;;
  *) printf 'warning: %s is not in PATH — add it to your shell profile\n' "$dir" >&2 ;;
esac

"$target" version

exit 0
} # end atomic brace group
