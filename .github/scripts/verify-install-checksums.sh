#!/usr/bin/env bash
# Verifies (or fixes) pinned download checksums in internal/image/tools/checksums.json
# and .github/msb.json.
#
# Two kinds of entries:
#  - pinned: checksum of a file at a version-scoped URL (nvm's install script, msb's
#    release checksums.sha256) - only changes when this repo bumps that pinned
#    version, so it's safe to check on every PR.
#  - live: checksum of a stable installer URL's *current* content, not tied to any
#    pinned tool version (claude/copilot/opencode) - can drift at any time
#    upstream, so --pinned-only skips these; only the daily scheduled refresh
#    workflow checks them, not per-PR CI.
#
# Usage: verify-install-checksums.sh [--fix] [--pinned-only] [path-to-versions.json] [path-to-checksums.json] [path-to-msb.json]
set -euo pipefail

# Checks one "<file> <key> <url>" entry against <file>'s <key>; --fix rewrites it in place.
check_one() {
  local file="$1" name="$2" url="$3"
  local expected actual

  expected=$(jq -r --arg n "$name" '.[$n]' "$file")
  actual=$(curl -fsSL "$url" | sha256sum | cut -d' ' -f1)

  if [[ -z "$actual" ]]; then
    echo "$name: failed to fetch $url" >&2
    return 1
  fi

  if [[ "$actual" = "$expected" ]]; then
    echo "$name checksum OK"
    return 0
  fi

  if [[ "$FIX" = false ]]; then
    echo "$name checksum in $file is stale"
    echo "  expected: $expected"
    echo "  actual:   $actual"
    return 1
  fi

  local tmp
  tmp=$(mktemp)
  jq --arg n "$name" --arg c "$actual" '.[$n] = $c' "$file" > "$tmp"
  mv "$tmp" "$file"
  echo "$name checksum updated to $actual"
}

# Runs check_one over a newline-separated "<file> <key> <url>" list, tracking failures in $status.
run_checks() {
  local entries="$1"
  while read -r file name url; do
    [[ -n "$name" ]] || continue
    check_one "$file" "$name" "$url" || status=1
  done <<< "$entries"
}

# --- main ---

FIX=false
PINNED_ONLY=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --fix) FIX=true; shift ;;
    --pinned-only) PINNED_ONLY=true; shift ;;
    *) break ;;
  esac
done

VERSIONS_FILE="${1:-internal/image/tools/versions.json}"
CHECKSUMS_FILE="${2:-internal/image/tools/checksums.json}"
MSB_FILE="${3:-.github/msb.json}"

NVM_VERSION=$(jq -r '.nvm' "$VERSIONS_FILE")
MSB_VERSION=$(jq -r '.version' "$MSB_FILE")

PINNED_ENTRIES="$CHECKSUMS_FILE nvm https://raw.githubusercontent.com/nvm-sh/nvm/v${NVM_VERSION}/install.sh
$MSB_FILE checksums_sha256 https://github.com/superradcompany/microsandbox/releases/download/${MSB_VERSION}/checksums.sha256"
LIVE_ENTRIES="$CHECKSUMS_FILE claude_install https://claude.ai/install.sh
$CHECKSUMS_FILE copilot_install https://gh.io/copilot-install
$CHECKSUMS_FILE opencode_install https://opencode.ai/v2/install"

status=0

run_checks "$PINNED_ENTRIES"
[[ "$PINNED_ONLY" = true ]] || run_checks "$LIVE_ENTRIES"

exit "$status"
