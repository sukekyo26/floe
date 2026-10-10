#!/usr/bin/env sh
# install.sh — fetch a floe release and drop it into your PATH.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/main/install.sh | sh
#
# Environment overrides:
#   FLOE_VERSION      Version to install (e.g. "0.1.0"); default "latest".
#   FLOE_INSTALL_DIR  Where to put the binary; default "$HOME/.local/bin".
#
# The binary's SHA-256 is verified against SHA256SUMS from the same release.

set -eu

REPO=sukekyo26/floe
INSTALL_DIR="${FLOE_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${FLOE_VERSION:-latest}"

die() {
  printf 'floe-install: %s\n' "$*" >&2
  exit 1
}

for tool in curl uname mktemp; do
  command -v "$tool" >/dev/null 2>&1 || die "missing required tool: $tool"
done

# Linux ships sha256sum; macOS ships shasum, which prints the same format under -a 256.
if command -v sha256sum >/dev/null 2>&1; then
  sha256() { sha256sum "$@"; }
elif command -v shasum >/dev/null 2>&1; then
  sha256() { shasum -a 256 "$@"; }
else
  die "missing required tool: sha256sum or shasum"
fi

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux | darwin) ;;
  *) die "unsupported OS: $os (floe supports linux and darwin)" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) die "unsupported architecture: $arch" ;;
esac

if [ "$VERSION" = "latest" ]; then
  # Capture first: POSIX sh has no pipefail, so a failed curl would be masked by the pipeline.
  releases_url="https://api.github.com/repos/$REPO/releases/latest"
  api_response=$(curl -fsSL --proto '=https' --tlsv1.2 "$releases_url") ||
    die "failed to fetch release metadata: $releases_url"
  tag=$(printf '%s' "$api_response" | sed -n 's/.*"tag_name": *"\(v\{0,1\}[^"]*\)".*/\1/p' | head -n1)
  [ -n "$tag" ] || die "could not parse tag_name from $releases_url"
else
  case "$VERSION" in
    v*) tag="$VERSION" ;;
    *) tag="v$VERSION" ;;
  esac
fi

asset="floe-$os-$arch"
base="https://github.com/$REPO/releases/download/$tag"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

printf 'floe-install: downloading %s@%s\n' "$asset" "${tag#v}" >&2
curl -fsSL --proto '=https' --tlsv1.2 "$base/$asset" -o "$tmp/$asset" || die "download failed: $base/$asset"
curl -fsSL --proto '=https' --tlsv1.2 "$base/SHA256SUMS" -o "$tmp/SHA256SUMS" || die "download failed: $base/SHA256SUMS"

expected=$(grep "  $asset\$" "$tmp/SHA256SUMS" | awk '{print $1}')
[ -n "$expected" ] || die "$asset not listed in SHA256SUMS"
actual=$(sha256 "$tmp/$asset" | awk '{print $1}')
[ "$expected" = "$actual" ] || die "checksum mismatch ($actual != $expected)"

mkdir -p "$INSTALL_DIR"
mv "$tmp/$asset" "$INSTALL_DIR/floe"
chmod +x "$INSTALL_DIR/floe"

printf 'floe-install: installed floe %s to %s/floe\n' "${tag#v}" "$INSTALL_DIR" >&2
case ":$PATH:" in
  *:"$INSTALL_DIR":*) ;;
  *) printf 'floe-install: note: %s is not in PATH; add it to ~/.profile or ~/.zshrc\n' "$INSTALL_DIR" >&2 ;;
esac
