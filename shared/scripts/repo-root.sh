# Resolves the absolute path to the repository root.
# Source this file to get $REPO_ROOT in your script.
# Usage: source "$(dirname "$0")/../shared/scripts/repo-root.sh"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
