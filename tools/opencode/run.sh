#!/usr/bin/env bash
set -euo pipefail

# --- Environment ---
# Enable exec on /tmp (required to run language server binaries).
TMPFS_EXEC_TMP=1

# --- Dependencies ---
# shellcheck source=../../shared/scripts/repo-root.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../shared/scripts/repo-root.sh"
# shellcheck source=../../shared/scripts/run-common.sh
source "${REPO_ROOT}/shared/scripts/run-common.sh"
# shellcheck source=./config.sh
source "$(dirname "${BASH_SOURCE[0]}")/config.sh"

# --- Pre-flight checks ---
check_image
check_git_repo

# --- Setup ---
mkdir -p \
  "${TOOL_HOME}/opencode/data" \
  "${TOOL_HOME}/opencode/cache" \
  "${TOOL_HOME}/opencode/state"

# --- Config ---
resolve_container_home # resolves CONTAINER_HOME

# --- Mounts ---
# Volume mounts, mount only what the tool needs:
#
#   - workspace : actual code base that needs to be worked on;
#   - data      : opencode directory containing shared configuration.
#   - cache     : opencode directory containing LSP indexes, models, etc..
#   - state     : opencode directory containing shared runtime state/logs.
MOUNTS+=(
  -v "${PWD}:/workspace"
  -v "${TOOL_HOME}/opencode/data:${CONTAINER_HOME}/.local/share/opencode"
  -v "${TOOL_HOME}/opencode/cache:${CONTAINER_HOME}/.cache/opencode"
  -v "${TOOL_HOME}/opencode/state:${CONTAINER_HOME}/.local/state/opencode"
)

# --- Run ---
run_container "$@"
