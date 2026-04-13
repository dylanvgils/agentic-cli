#!/usr/bin/env bash
set -euo pipefail

BIN_DIR="$HOME/.local/bin"
BINARY="agentic"

if [[ ! -L "$BIN_DIR/$BINARY" ]]; then
  echo "No symlink found at $BIN_DIR/$BINARY, nothing to do."
  exit 0
fi

rm "$BIN_DIR/$BINARY"
echo "Removed $BIN_DIR/$BINARY"
