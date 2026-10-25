#!/usr/bin/env bash
# Reports a failed nightly audit: comments on the open security-audit issue, or
# opens one, so repeated failures land on a single issue until it's closed.
#
# Usage: report-audit.sh <audit-log>
set -euo pipefail

LABEL="security-audit"
MAX_LOG_BYTES=60000 # GitHub caps issue/comment bodies at 65536 chars

log_file="$1"
run_url="$GITHUB_SERVER_URL/$GITHUB_REPOSITORY/actions/runs/$GITHUB_RUN_ID"

body_file="$(mktemp)"
trap 'rm -f "$body_file"' EXIT

{
  echo "Nightly audit failed: $run_url"
  echo
  echo '```'
  tail -c "$MAX_LOG_BYTES" "$log_file"
  echo '```'
} > "$body_file"

issue="$(gh issue list --label "$LABEL" --state open --limit 1 --json number --jq '.[0].number // empty')"

if [[ -n "$issue" ]]; then
  gh issue comment "$issue" --body-file "$body_file"
else
  gh label create "$LABEL" --force --color B60205 --description "Findings from the nightly audit workflow"
  gh issue create --title "Nightly audit found issues" --label "$LABEL" --body-file "$body_file"
fi
