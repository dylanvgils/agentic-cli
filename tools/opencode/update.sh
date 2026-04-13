#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=./config.sh
source "$(dirname "${BASH_SOURCE[0]}")/config.sh"
# shellcheck source=../../shared/scripts/update-common.sh
source "${REPO_ROOT}/shared/scripts/update-common.sh"

update_tool "$(dirname "${BASH_SOURCE[0]}")"
