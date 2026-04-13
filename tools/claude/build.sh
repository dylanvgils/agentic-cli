#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=./config.sh
source "$(dirname "${BASH_SOURCE[0]}")/config.sh"
# shellcheck source=../../shared/scripts/build-common.sh
source "${REPO_ROOT}/shared/scripts/build-common.sh"

build_tool "$(dirname "${BASH_SOURCE[0]}")"
