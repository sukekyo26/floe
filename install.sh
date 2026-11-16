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

# path_note says how to put the install directory on PATH, when it is not there. It is called with floe in place:
# nothing in it fails the install, and when a sed of it fails it falls back to words alone.
# LIMIT: the file is named for zsh and for bash on Linux alone, whose interactive shells always read it; a bash that
# only ever runs as a login shell and whose ~/.bash_profile does not read ~/.bashrc misses the line. A zsh with
# noclobber and no ~/.zshrc yet refuses the >> (it says so). fish older than 3.2 has no fish_add_path (it says so).
# The line is added at the front of PATH each time the file is read, so a shell started in a shell has it twice.
# Control characters are looked for byte by byte: a UTF-8 C1 one (U+0085, U+009B: C2 9B) is not refused by dash or a
# bash under LC_ALL=C, nor turned into ? by tr. FLOE_INSTALL_DIR is the user's own; look at the characters, not the
# bytes, if a directory is ever named by someone else.
path_note() {
  # ./ in front of a relative one: cd then reads neither CDPATH nor a name like - or -x as its own.
  case "$INSTALL_DIR" in
    /*) abs=$INSTALL_DIR ;;
    *) abs=./$INSTALL_DIR ;;
  esac
  abs=$(cd "$abs" 2>/dev/null && pwd) || abs=$INSTALL_DIR
  # The name as given is looked at too: $(...) drops the line breaks it ends in. A control character (a CR going back
  # to the start of the line) would also make the line shown on screen another than the one pasted.
  case "$abs" in
    *:* | [!/]*) bad=1 ;;
    *) bad= ;;
  esac
  case "$abs$INSTALL_DIR" in
    *[[:cntrl:]]*) bad=1 ;;
  esac
  if [ -n "$bad" ]; then
    printf 'floe-install: note: %s cannot be a PATH entry (PATH takes an absolute path without ":" or a control character); run floe by its full path, or set FLOE_INSTALL_DIR to another directory\n' "$(printf '%s' "$INSTALL_DIR" | tr '[:cntrl:]' '?')" >&2
    return 0
  fi
  case ":$PATH:" in
    *:"$abs":* | *:"$abs/":*) return 0 ;;
  esac
  home=${HOME-}
  home=${home%/}
  shell=${SHELL-}
  shell=${shell##*/}
  file=
  case "$shell" in
    fish) how=fish ;;
    zsh)
      how=posix
      # shellcheck disable=SC2016 # shown as it is: the shell it is pasted into opens it
      file='"${ZDOTDIR:-$HOME}/.zshrc"'
      ;;
    bash)
      how=posix
      # A login bash, which a macOS terminal starts, does not read ~/.bashrc.
      # shellcheck disable=SC2088 # shown as it is
      [ "$os" != linux ] || file='~/.bashrc'
      ;;
    sh | dash | ash | ksh | mksh) how=posix ;;
    *) how= ;;
  esac
  # Each sed on its own line, out of quotes: a sed that fails (none on PATH, or one that refuses the bytes) leaves
  # words alone, not a line that adds "" or the working directory to PATH.
  case "$how" in
    fish)
      fished=$(printf '%s\n' "$abs" | sed "s/[\\\\']/\\\\&/g") && [ -n "$fished" ] || how=
      ;;
    posix)
      prefix='' rest=$abs
      if [ -n "$home" ]; then
        # shellcheck disable=SC2016 # written into the file as $HOME
        case "$abs" in "$home"/*) prefix='$HOME' rest=${abs#"$home"} ;; esac
      fi
      # In the file the directory is inside double quotes, where \ " $ ` are not themselves.
      escaped=$(printf '%s\n' "$rest" | sed 's/[\\"$`]/\\&/g') && [ -n "$escaped" ] || how=
      line="export PATH=\"$prefix$escaped:\$PATH\""
      quoted=$(printf '%s\n' "$line" | sed "s/'/'\\\\''/g") && [ -n "$quoted" ] || how=
      ;;
  esac
  case "$how" in
    fish)
      printf 'floe-install: %s is not on your PATH. To add it, run:\n' "$abs" >&2
      printf "  fish_add_path '%s'\n" "$fished" >&2
      ;;
    posix)
      if [ -n "$file" ]; then
        printf 'floe-install: %s is not on your PATH. To add it, run:\n' "$abs" >&2
        # The empty line first: a file whose last line has no line break would take ours onto that line.
        printf "  printf '\\\\n%%s\\\\n' '%s' >> %s\n" "$quoted" "$file" >&2
      else
        printf "floe-install: %s is not on your PATH. To add it, put this line in your shell's startup file:\n" "$abs" >&2
        printf '  %s\n' "$line" >&2
      fi
      printf 'floe-install: then open a new terminal\n' >&2
      ;;
    *) printf 'floe-install: note: %s is not on your PATH; add it to the PATH your shell starts with\n' "$abs" >&2 ;;
  esac
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
  *) die "unsupported architecture: $arch (floe supports amd64 and arm64)" ;;
esac

if [ "$VERSION" = "latest" ]; then
  # Capture first: POSIX sh has no pipefail, so a failed curl would be masked by the pipeline.
  releases_url="https://api.github.com/repos/$REPO/releases/latest"
  api_response=$(curl -fsSL --proto '=https' --tlsv1.2 "$releases_url") ||
    die "failed to fetch release metadata: $releases_url (no release yet, or GitHub's API rate limit); try again later, or set FLOE_VERSION to a version, which skips the API"
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

cannot_write="cannot write to $INSTALL_DIR; set FLOE_INSTALL_DIR to a directory you can write to"
mkdir -p "$INSTALL_DIR" || die "$cannot_write"
mv "$tmp/$asset" "$INSTALL_DIR/floe" || die "$cannot_write"
chmod +x "$INSTALL_DIR/floe"

printf 'floe-install: installed floe %s to %s/floe\n' "${tag#v}" "$INSTALL_DIR" >&2
path_note || :
printf 'floe-install: for your agent: run floe skill install <agent> (floe skill install --help lists them)\n' >&2
