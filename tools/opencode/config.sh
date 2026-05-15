# This file is sourced by build.sh and update.sh - do not execute directly.

# shellcheck source=../../shared/scripts/repo-root.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../shared/scripts/repo-root.sh"
# shellcheck source=../../shared/config.sh
source "${REPO_ROOT}/shared/config.sh"

BASE=""
IMAGE="agentic-opencode"
VERSION_CMD="opencode --version"
