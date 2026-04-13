#!/usr/bin/env bash
set -euo pipefail

# Set GITHUB_TOKEN if mounted in container
if [[ -f /run/secrets/copilot_token ]]; then
  export GITHUB_TOKEN="$(cat /run/secrets/copilot_token)"
fi

exec copilot "$@"
