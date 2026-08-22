#!/usr/bin/env bash
# Verifies (or fixes) nvm_checksum in internal/tools/versions.json against the
# sha256 of the pinned nvm version's install.sh, so a version bump can't
# silently leave a stale checksum behind.
# Usage: verify-nvm-checksum.sh [--fix] [path-to-versions.json]
set -euo pipefail

FIX=false
if [ "${1:-}" = "--fix" ]; then
  FIX=true
  shift
fi

VERSIONS_FILE="${1:-internal/tools/versions.json}"

NVM_VERSION=$(jq -r '.nvm' "$VERSIONS_FILE")
EXPECTED_CHECKSUM=$(jq -r '.nvm_checksum' "$VERSIONS_FILE")
ACTUAL_CHECKSUM=$(curl -fsSL "https://raw.githubusercontent.com/nvm-sh/nvm/v${NVM_VERSION}/install.sh" | sha256sum | cut -d' ' -f1)

if [ "$ACTUAL_CHECKSUM" = "$EXPECTED_CHECKSUM" ]; then
  echo "nvm_checksum OK for nvm v${NVM_VERSION}"
  exit 0
fi

if [ "$FIX" = false ]; then
  echo "nvm_checksum in $VERSIONS_FILE is stale for nvm v${NVM_VERSION}"
  echo "  expected: $EXPECTED_CHECKSUM"
  echo "  actual:   $ACTUAL_CHECKSUM"
  exit 1
fi

TMP=$(mktemp)
jq --arg checksum "$ACTUAL_CHECKSUM" '.nvm_checksum = $checksum' "$VERSIONS_FILE" > "$TMP"
mv "$TMP" "$VERSIONS_FILE"
echo "nvm_checksum updated to $ACTUAL_CHECKSUM for nvm v${NVM_VERSION}"
