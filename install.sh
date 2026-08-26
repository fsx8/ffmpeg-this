#!/usr/bin/env bash
# ffwiz installer for apt-based systems (Ubuntu, Debian, and derivatives).
#
# One-liner:
#   curl -fsSL https://fsx8.github.io/ffwiz/install.sh | sudo bash
#
# What it does:
#   1. Installs the repository signing key (binary keyring, no gpg needed)
#   2. Adds the ffwiz apt repository (GPG-verified)
#   3. Runs apt update and installs (or upgrades) ffwiz
set -euo pipefail

readonly REPO_URL="https://fsx8.github.io/ffwiz/apt"
readonly KEY_URL="https://fsx8.github.io/ffwiz/ffwiz-archive-keyring.gpg"
readonly SCRIPT_URL="https://fsx8.github.io/ffwiz/install.sh"
readonly KEYRING="/usr/share/keyrings/ffwiz-archive-keyring.gpg"
readonly SOURCES_FILE="/etc/apt/sources.list.d/ffwiz.list"
readonly ARCHES="amd64,arm64"

log()  { printf '[ffwiz] %s\n' "$*"; }
fail() { printf '[ffwiz] error: %s\n' "$*" >&2; exit 1; }

if [ "$(id -u)" -ne 0 ]; then
  if command -v sudo >/dev/null 2>&1; then
    # When piped (`curl ... | bash`) $0 is unusable; re-fetch instead.
    if [ -f "$0" ]; then
      exec sudo bash "$0" "$@"
    else
      exec sudo bash -c "$(curl -fsSL "$SCRIPT_URL")"
    fi
  fi
  fail "must run as root — try: curl -fsSL ${SCRIPT_URL} | sudo bash"
fi

command -v apt-get >/dev/null 2>&1 || fail "apt-get not found; this installer is for Debian/Ubuntu systems."
command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1 || fail "need curl or wget to download the key."

ARCH="$(dpkg --print-architecture 2>/dev/null || uname -m)"
case "$ARCH" in
  amd64|arm64) ;;
  *) fail "unsupported architecture '${ARCH}' — ffwiz is published for amd64 and arm64." ;;
esac

log "installing repository signing key to ${KEYRING}"
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$KEY_URL" -o "$KEYRING"
else
  wget -qO "$KEYRING" "$KEY_URL"
fi
chmod 0644 "$KEYRING"

log "adding apt repository to ${SOURCES_FILE}"
printf 'deb [arch=%s signed-by=%s] %s stable main\n' "$ARCHES" "$KEYRING" "$REPO_URL" > "$SOURCES_FILE"

log "running apt update"
apt-get update -o Acquire::Retries=3

log "installing ffwiz"
apt-get install -y ffwiz

if ! command -v ffmpeg >/dev/null 2>&1; then
  printf '[ffwiz] note: ffmpeg was not found in PATH — ffwiz needs it at runtime.\n' >&2
  printf '[ffwiz] note: install it with: sudo apt-get install -y ffmpeg\n' >&2
fi

log "installed: $(command -v ffwiz) ($(ffwiz -version 2>/dev/null || echo unknown version))"
