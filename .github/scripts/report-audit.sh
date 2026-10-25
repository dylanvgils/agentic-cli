#!/usr/bin/env bash
# Reports a failed nightly audit: comments on the open security-audit issue, or
# opens one, so repeated failures land on a single issue until it's closed.
#
# Usage: report-audit.sh <audit-log> <govulncheck-json>
set -euo pipefail

LABEL="security-audit"
MAX_LOG_BYTES=50000 # GitHub caps issue/comment bodies at 65536 chars, leave room for the table

log_file="$1"
vulns_file="$2"
run_url="$GITHUB_SERVER_URL/$GITHUB_REPOSITORY/actions/runs/$GITHUB_RUN_ID"

body_file="$(mktemp)"
trap 'rm -f "$body_file"' EXIT

# One row per vulnerability our code calls (trace[0] is the vulnerable symbol).
rows="$(jq -rs '
  (map(select(.osv)) | map({key: .osv.id, value: .osv.summary}) | from_entries) as $summaries
  | map(.finding | select(. != null and (.trace[0].function // "") != ""))
  | group_by(.osv) | reverse | .[][0]
  | .trace[0] as $sym
  | (if $sym.module == "stdlib" then "go" else "" end) as $prefix
  | [
      "[\(.osv)](https://pkg.go.dev/vuln/\(.osv))",
      ($summaries[.osv] // "" | gsub("\\|"; "\\|")),
      "`\($sym.package // $sym.module)`",
      $prefix + ($sym.version // "?" | if $prefix != "" then ltrimstr("v") else . end),
      if .fixed_version then $prefix + (.fixed_version | if $prefix != "" then ltrimstr("v") else . end) else "not fixed" end
    ]
  | "| " + join(" | ") + " |"
' "$vulns_file" 2>/dev/null || true)"

{
  echo "## Nightly audit failed"
  echo
  echo "[Workflow run]($run_url)"
  echo

  if [[ -n "$rows" ]]; then
    echo "### Vulnerabilities called by our code ($(wc -l <<< "$rows"))"
    echo
    echo "| ID | Summary | Package | Found | Fixed in |"
    echo "| --- | --- | --- | --- | --- |"
    echo "$rows"
  else
    echo "No vulnerabilities called by our code; see the log for what failed."
  fi

  echo
  echo "<details>"
  echo "<summary>Full audit log</summary>"
  echo
  echo '```'
  tail -c "$MAX_LOG_BYTES" "$log_file"
  echo '```'
  echo
  echo "</details>"
} > "$body_file"

issue="$(gh issue list --label "$LABEL" --state open --limit 1 --json number --jq '.[0].number // empty')"

if [[ -n "$issue" ]]; then
  gh issue comment "$issue" --body-file "$body_file"
else
  gh label create "$LABEL" --force --color B60205 --description "Findings from the nightly audit workflow"
  gh issue create --title "Nightly audit found issues" --label "$LABEL" --body-file "$body_file"
fi
