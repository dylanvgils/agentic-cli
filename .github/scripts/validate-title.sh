#!/usr/bin/env bash
# Validates that a string follows the Conventional Commits format.
# Usage: validate-title.sh <title>
set -e

TITLE="${1}"
PATTERN='^(feat|fix|chore|docs|style|refactor|perf|test|build|ci|revert)(\([^)]+\))?!?: .+'

if ! echo "$TITLE" | grep -qE "$PATTERN"; then
  echo "PR title does not follow conventional commits: $TITLE"
  exit 1
fi
