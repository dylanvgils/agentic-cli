#!/usr/bin/env bash
set -euo pipefail

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
mkdir -p "${TOOL_HOME}/claude/data"

[[ -f "${TOOL_HOME}/claude/.claude.json" ]] ||
  echo '{}' >"${TOOL_HOME}/claude/.claude.json"

# --- Config ---
resolve_container_home # resolves CONTAINER_HOME

# --- Mounts ---
# Mount only what the tool needs:
#
#   - workspace : actual code base that needs to be worked on;
#   - data      : claude directory containing shared configuration.
MOUNTS+=(
  -v "${PWD}:/workspace"
  -v "${TOOL_HOME}/claude/data:${CONTAINER_HOME}/.claude"
  -v "${TOOL_HOME}/claude/.claude.json:${CONTAINER_HOME}/.claude.json"
)

# --- Run ---
run_container "$@"
