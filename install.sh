#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN_DIR="$HOME/.local/bin"
BINARY="agentic"

mkdir -p "$BIN_DIR"

if [[ -e "$BIN_DIR/$BINARY" || -L "$BIN_DIR/$BINARY" ]]; then
  echo "Already installed at $BIN_DIR/$BINARY - run uninstall.sh first."
  exit 1
fi

ln -s "$REPO_DIR/bin/$BINARY" "$BIN_DIR/$BINARY"
echo "Installed $BINARY -> $BIN_DIR/$BINARY"

(cd "$REPO_DIR" && make install)

if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
  echo ""
  echo "Warning: $BIN_DIR is not in your PATH."
  echo "Add the following to your shell profile (e.g. ~/.zshrc, ~/.bashrc):"
  echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
fi
