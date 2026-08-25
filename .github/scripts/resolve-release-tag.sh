#!/usr/bin/env bash
# Resolves the tag to release for the release workflow, from either an
# explicit workflow_dispatch input or an auto-computed next version.
#
# Prints GITHUB_OUTPUT-style key=value lines on stdout:
#   release_tag  - the tag to build and release (unset if skip=true)
#   previous_tag - the tag immediately before release_tag, pinning GoReleaser's changelog
#                  range instead of letting it re-derive "previous tag" on its own;
#                  empty when there is no real previous tag (unset if skip=true)
#   create       - "true" if release_tag still needs to be created (git tag + push)
#                  before releasing, "false" if it already exists (unset if skip=true)
#   skip         - "true" if there is nothing new to release; no other output is set
# All other messages go to stderr, so stdout can be redirected straight into GITHUB_OUTPUT.
# Usage: resolve-release-tag.sh <tag-input> <event-name>
set -e

TAG_INPUT="${1:-}"
EVENT_NAME="${2:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

LAST=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
echo "event=$EVENT_NAME tag_input='$TAG_INPUT' last=$LAST" >&2

# Prints "previous_tag=<tag>", or an empty value if <tag> isn't a real ref
# (e.g. LAST fell back to the synthetic v0.0.0 because no tags exist yet).
# GoReleaser treats an empty GORELEASER_PREVIOUS_TAG as "no previous tag".
print_previous_tag() {
  if git rev-parse "$1" >/dev/null 2>&1; then
    echo "previous_tag=$1"
  else
    echo "previous_tag="
  fi
}

if [ "$EVENT_NAME" = "schedule" ] || { [ "$EVENT_NAME" = "workflow_dispatch" ] && [ -z "$TAG_INPUT" ]; }; then
  NEXT=$("$SCRIPT_DIR/next-tag.sh" "$LAST")
  echo "auto-computed next tag is '${NEXT:-<none>}'" >&2

  if [ -z "$NEXT" ]; then
    echo "nothing to release since $LAST" >&2
    echo "skip=true"
    exit 0
  fi

  echo "release_tag=$NEXT"
  print_previous_tag "$LAST"
  echo "create=true"
  echo "skip=false"
  exit 0
fi

if ! echo "$TAG_INPUT" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "invalid tag '$TAG_INPUT', expected format vX.Y.Z" >&2
  exit 1
fi

if [ "$TAG_INPUT" = "$LAST" ]; then
  # Re-releasing the current latest tag: either it's already fully released
  # (goreleaser will error "already exists"), or a previous run failed
  # partway through publishing it. Either way, this is an explicit request
  # to redo it. The changelog's previous tag is one further back than LAST,
  # since LAST is the tag being re-released: describe from LAST's parent
  # commit to find the nearest tag before it.
  PREV_OF_LAST=$(git describe --tags --abbrev=0 "${LAST}^" 2>/dev/null || echo "")
  echo "re-releasing current latest tag $TAG_INPUT (previous=$PREV_OF_LAST)" >&2
  echo "release_tag=$TAG_INPUT"
  print_previous_tag "$PREV_OF_LAST"
  echo "create=false"
  echo "skip=false"
  exit 0
fi

if git rev-parse "$TAG_INPUT" >/dev/null 2>&1; then
  echo "tag $TAG_INPUT already exists" >&2
  exit 1
fi

NEWEST=$(printf '%s\n%s\n' "$LAST" "$TAG_INPUT" | sort -V | tail -n1)
if [ "$NEWEST" != "$TAG_INPUT" ]; then
  echo "tag $TAG_INPUT is not newer than the current latest tag $LAST" >&2
  exit 1
fi

echo "release_tag=$TAG_INPUT"
print_previous_tag "$LAST"
echo "create=true"
echo "skip=false"
