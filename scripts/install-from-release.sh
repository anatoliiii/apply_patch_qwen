#!/usr/bin/env bash
#
# One-line installer for apply_patch_qwen.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/anatoliiii/apply_patch_qwen/main/scripts/install-from-release.sh | sh
#
# Pin a version:
#   VERSION=v1.0.0 curl -fsSL https://raw.githubusercontent.com/anatoliiii/apply_patch_qwen/main/scripts/install-from-release.sh | sh
#
set -euo pipefail

REPO="anatoliiii/apply_patch_qwen"
VERSION="${VERSION:-latest}"
TMPDIR_ROOT="${TMPDIR:-/tmp}"

detect_os() {
  local os
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    linux)  echo "linux" ;;
    darwin) echo "darwin" ;;
    *)
      echo "Unsupported OS: $os" >&2
      exit 1
      ;;
  esac
}

detect_arch() {
  local arch
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)   echo "amd64" ;;
    aarch64|arm64)   echo "arm64" ;;
    *)
      echo "Unsupported architecture: $arch" >&2
      exit 1
      ;;
  esac
}

resolve_version() {
  if [[ "$VERSION" == "latest" ]]; then
    local resolved
    resolved="$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" 2>/dev/null || true)"
    if [[ -z "$resolved" ]]; then
      echo "Failed to resolve latest release version." >&2
      echo "Set VERSION=vX.Y.Z explicitly and retry." >&2
      exit 1
    fi
    VERSION="$(basename "$resolved")"
  fi
  echo "$VERSION"
}

main() {
  local os arch version url tmpdir

  os="$(detect_os)"
  arch="$(detect_arch)"
  version="$(resolve_version)"

  echo "apply_patch_qwen installer"
  echo "  Version: $version"
  echo "  OS:      $os"
  echo "  Arch:    $arch"
  echo

  url="https://github.com/${REPO}/releases/download/${version}/apply_patch_qwen_${version}_${os}_${arch}.tar.gz"

  tmpdir="$(mktemp -d "${TMPDIR_ROOT}/apply_patch_qwen_install.XXXXXX")"
  trap 'rm -rf "$tmpdir"' EXIT

  echo "Downloading ${url}..."
  curl -fsSL "$url" -o "$tmpdir/release.tar.gz"

  echo "Extracting..."
  tar -xzf "$tmpdir/release.tar.gz" -C "$tmpdir"

  # Find and run install.sh from the extracted archive
  local install_script
  install_script="$(find "$tmpdir" -name install.sh -type f | head -1)"

  if [[ -z "$install_script" ]]; then
    echo "Error: install.sh not found in release archive." >&2
    exit 1
  fi

  chmod +x "$install_script"
  echo "Running installer..."
  echo
  bash "$install_script"
}

main "$@"
