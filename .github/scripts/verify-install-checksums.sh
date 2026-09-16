#!/usr/bin/env bash
# Verifies (or fixes) installer-script checksums in internal/tools/checksums.json.
#
# Two kinds of entries:
#  - pinned: checksum of a specific pinned tool version's install script, at a
#    version-scoped URL (currently just nvm) - only changes when this repo bumps
#    that pinned version, so it's safe to check on every PR.
#  - live: checksum of a stable installer URL's *current* content, not tied to any
#    pinned tool version (claude/copilot/opencode) - can drift at any time
#    upstream, so --pinned-only skips these; only the daily scheduled refresh
#    workflow checks them, not per-PR CI.
#
# Usage: verify-install-checksums.sh [--fix] [--pinned-only] [path-to-versions.json] [path-to-checksums.json]
set -euo pipefail

FIX=false
PINNED_ONLY=false
while [ $# -gt 0 ]; do
  case "$1" in
    --fix) FIX=true; shift ;;
    --pinned-only) PINNED_ONLY=true; shift ;;
    *) break ;;
  esac
done

VERSIONS_FILE="${1:-internal/tools/versions.json}"
CHECKSUMS_FILE="${2:-internal/tools/checksums.json}"

NVM_VERSION=$(jq -r '.nvm' "$VERSIONS_FILE")

PINNED_ENTRIES="nvm https://raw.githubusercontent.com/nvm-sh/nvm/v${NVM_VERSION}/install.sh"
LIVE_ENTRIES="claude_install https://claude.ai/install.sh
copilot_install https://gh.io/copilot-install
opencode_install https://opencode.ai/install"

check_one() {
  local name="$1" url="$2"
  local expected actual

  expected=$(jq -r --arg n "$name" '.[$n]' "$CHECKSUMS_FILE")
  actual=$(curl -fsSL "$url" | sha256sum | cut -d' ' -f1)

  if [ -z "$actual" ]; then
    echo "$name: failed to fetch $url" >&2
    return 1
  fi

  if [ "$actual" = "$expected" ]; then
    echo "$name checksum OK"
    return 0
  fi

  if [ "$FIX" = false ]; then
    echo "$name checksum in $CHECKSUMS_FILE is stale"
    echo "  expected: $expected"
    echo "  actual:   $actual"
    return 1
  fi

  local tmp
  tmp=$(mktemp)
  jq --arg n "$name" --arg c "$actual" '.[$n] = $c' "$CHECKSUMS_FILE" > "$tmp"
  mv "$tmp" "$CHECKSUMS_FILE"
  echo "$name checksum updated to $actual"
}

status=0

while read -r name url; do
  [ -n "$name" ] || continue
  check_one "$name" "$url" || status=1
done <<< "$PINNED_ENTRIES"

if [ "$PINNED_ONLY" = false ]; then
  while read -r name url; do
    [ -n "$name" ] || continue
    check_one "$name" "$url" || status=1
  done <<< "$LIVE_ENTRIES"
fi

exit "$status"
