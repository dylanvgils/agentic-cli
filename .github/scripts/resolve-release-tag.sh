#!/usr/bin/env bash
# Resolves the tag to release for the release workflow, from either an
# explicit workflow_dispatch input or an auto-computed next version.
# Prints GITHUB_OUTPUT-style key=value lines on stdout: release_tag, create, skip.
# All other messages go to stderr, so stdout can be redirected straight into GITHUB_OUTPUT.
# Usage: resolve-release-tag.sh <tag-input> <event-name>
set -e

TAG_INPUT="${1:-}"
EVENT_NAME="${2:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

LAST=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")

if [ "$EVENT_NAME" = "schedule" ] || { [ "$EVENT_NAME" = "workflow_dispatch" ] && [ -z "$TAG_INPUT" ]; }; then
  NEXT=$("$SCRIPT_DIR/next-tag.sh" "$LAST")

  if [ -z "$NEXT" ]; then
    if [ "$EVENT_NAME" = "schedule" ]; then
      echo "nothing to release since $LAST" >&2
      echo "skip=true"
      exit 0
    fi
    echo "no releasable commits since $LAST, nothing to release" >&2
    exit 1
  fi

  echo "release_tag=$NEXT"
  echo "create=true"
  echo "skip=false"
  exit 0
fi

if [ "$TAG_INPUT" = "latest" ]; then
  echo "release_tag=$LAST"
  echo "create=false"
  echo "skip=false"
  exit 0
fi

if ! echo "$TAG_INPUT" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "invalid tag '$TAG_INPUT', expected format vX.Y.Z" >&2
  exit 1
fi

if git rev-parse "$TAG_INPUT" >/dev/null 2>&1; then
  echo "tag $TAG_INPUT already exists" >&2
  exit 1
fi

NEWEST=$(printf '%s\n%s\n' "$LAST" "$TAG_INPUT" | sort -V | tail -n1)
if [ "$NEWEST" != "$TAG_INPUT" ] || [ "$TAG_INPUT" = "$LAST" ]; then
  echo "tag $TAG_INPUT is not newer than the current latest tag $LAST" >&2
  exit 1
fi

echo "release_tag=$TAG_INPUT"
echo "create=true"
echo "skip=false"
