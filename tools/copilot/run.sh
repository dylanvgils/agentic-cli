#!/usr/bin/env bash
set -euo pipefail

# --- Environment ---
# Enable exec on /tmp (allow to run compiled output).
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
mkdir -p "${TOOL_HOME}/copilot"

# --- Config ---
resolve_container_home # resolves CONTAINER_HOME

# --- Mounts ---
TMPFS_FLAGS+=(--tmpfs "${CONTAINER_HOME}/.cache:exec,size=1g")

# Volume mounts, mount only what the tool needs:
#
#   - workspace       : actual code base that needs to be worked on;
#   - copilot config  : copilot directory containing shared configuration.
MOUNTS+=(
  -v "${PWD}:/workspace"
  -v "${TOOL_HOME}/copilot:${CONTAINER_HOME}/.copilot"
)

# Optionally mount the copilot token so the container can reuse the
# host session, only if the token file exists on the host.
if [[ -f "${HOME}/.secrets/copilot_token" ]]; then
  MOUNTS+=(-v "${HOME}/.secrets/copilot_token:/run/secrets/copilot_token:ro")
fi

# --- Run ---
run_container "$@"
