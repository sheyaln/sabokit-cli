#!/usr/bin/env bash
set -euo pipefail

REPO="${SABOKIT_REPO:-sheyaln/sabokit-cli}"
VERSION="${SABOKIT_VERSION:-latest}"
INSTALL_DIR="${SABOKIT_INSTALL_DIR:-}"

die() { echo "error: $*" >&2; exit 1; }

detect_os() {
  case "$(uname -s)" in
    Linux)  echo linux ;;
    Darwin) echo darwin ;;
    *) die "unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  echo amd64 ;;
    arm64|aarch64) echo arm64 ;;
    *) die "unsupported arch: $(uname -m)" ;;
  esac
}

resolve_install_dir() {
  if [ -n "$INSTALL_DIR" ]; then
    echo "$INSTALL_DIR"
    return
  fi
  if [ -w /usr/local/bin ] 2>/dev/null; then
    echo /usr/local/bin
    return
  fi
  mkdir -p "$HOME/.local/bin"
  echo "$HOME/.local/bin"
}

main() {
  command -v curl >/dev/null || die "curl is required"

  os=$(detect_os)
  arch=$(detect_arch)
  asset="sabokit-${os}-${arch}"

  if [ "$VERSION" = "latest" ]; then
    url="https://github.com/${REPO}/releases/latest/download/${asset}"
  else
    url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
  fi

  dir=$(resolve_install_dir)
  tmp=$(mktemp)
  trap 'rm -f "$tmp"' EXIT

  echo "downloading ${url}"
  curl -fsSL --proto '=https' --tlsv1.2 -o "$tmp" "$url" \
    || die "download failed"

  install -m 0755 "$tmp" "${dir}/sabokit"
  echo "installed sabokit to ${dir}/sabokit"

  case ":$PATH:" in
    *":${dir}:"*) ;;
    *) echo "warning: ${dir} is not in PATH — add it to your shell profile" >&2 ;;
  esac

  "${dir}/sabokit" version
}

main "$@"
