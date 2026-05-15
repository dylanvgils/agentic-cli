#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN_DIR="$HOME/.local/bin"
BINARY="agentic"

if [[ ! -L "$BIN_DIR/$BINARY" ]]; then
  echo "No symlink found at $BIN_DIR/$BINARY, nothing to do."
  exit 0
fi

rm "$BIN_DIR/$BINARY"
echo "Removed $BIN_DIR/$BINARY"

(cd "$REPO_DIR" && make uninstall)
echo "Removed $BIN_DIR/$BINARY-cli"
