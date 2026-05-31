#!/usr/bin/env bash
# Prints the next semver tag based on conventional commits since the last tag,
# or exits 0 with no output if no releaseable commits are found.
# Usage: next-tag.sh [base-tag]  (base-tag defaults to the latest git tag)
set -e

LAST="${1:-$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")}"

COMMITS=$(git log "${LAST}..HEAD" --format="%s")
BODIES=$(git log "${LAST}..HEAD" --format="%b")

if [ -z "$COMMITS" ]; then
  exit 0
fi

# Determine the highest bump level across all commits since last tag.
# feat!: / fix!: / any type with ! -> major
# feat:                             -> minor
# fix: / perf: / refactor:          -> patch
# anything else (chore, docs, ...)  -> no release
BUMP="none"

while IFS= read -r msg; do
  if echo "$msg" | grep -qE '^[a-z]+(\([^)]+\))?!:'; then
    BUMP="major"; break
  elif echo "$msg" | grep -qE '^feat(\([^)]+\))?:'; then
    [ "$BUMP" != "major" ] && BUMP="minor"
  elif echo "$msg" | grep -qE '^(fix|perf|refactor)(\([^)]+\))?:'; then
    [ "$BUMP" = "none" ] && BUMP="patch"
  fi
done <<< "$COMMITS"

# Also catch breaking changes declared in the commit body footer.
if echo "$BODIES" | grep -q "^BREAKING CHANGE:"; then
  BUMP="major"
fi

if [ "$BUMP" = "none" ]; then
  exit 0
fi

VERSION="${LAST#v}"
MAJOR=$(echo "$VERSION" | cut -d. -f1)
MINOR=$(echo "$VERSION" | cut -d. -f2)
PATCH=$(echo "$VERSION" | cut -d. -f3)

case "$BUMP" in
  major) MAJOR=$((MAJOR+1)); MINOR=0; PATCH=0 ;;
  minor) MINOR=$((MINOR+1)); PATCH=0 ;;
  patch) PATCH=$((PATCH+1)) ;;
esac

echo "v${MAJOR}.${MINOR}.${PATCH}"
