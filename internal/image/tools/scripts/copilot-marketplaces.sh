marketplaces_dir="$HOME/{{.MarketplacesDir}}"

# Prints "NAME<tab>PATH" per local marketplace. copilot plugin marketplace list has no --json output, so this parses
# its "NAME (Local: PATH)" lines. Fragile: the text format is not a stable contract, so simplify (or drop the
# path-matching) once copilot ships machine-readable list output.
list_local_marketplaces() {
  copilot plugin marketplace list 2>/dev/null | awk '
    i = index($0, " (Local: ") {
      name = substr($0, 1, i - 1); sub(/^.*[ \t]/, "", name)
      loc = substr($0, i + 9); sub(/\)$/, "", loc)
      print name "\t" loc
    }'
}

# Register AGENTIC_MARKETPLACES into Copilot
# Not idempotent, so skip marketplaces already registered (e.g. from a previous run against the same persisted
# $HOME/.copilot mount). Matched by local path rather than name, since copilot derives its name from the manifest.
if [[ -n "${AGENTIC_MARKETPLACES:-}" ]]; then
  registered_locs="$(list_local_marketplaces | cut -f2)"
  IFS=',' read -ra marketplace_names <<< "$AGENTIC_MARKETPLACES"
  for name in "${marketplace_names[@]}"; do
    dir="$marketplaces_dir/$name"
    [[ -d "$dir" ]] || continue
    grep -qxF "$dir" <<< "$registered_locs" && continue
    copilot plugin marketplace add "$dir" || echo "warning: failed to register marketplace $name" >&2
  done
fi

# Deregister marketplaces this container no longer mounts, e.g. removed from .agenticrc.toml
while IFS=$'\t' read -r name loc; do
  [[ -n "$name" && "$(dirname -- "$loc")" == "$marketplaces_dir" && ! -d "$loc" ]] || continue
  copilot plugin marketplace remove --force "$name" || echo "warning: failed to deregister marketplace $name" >&2
done < <(list_local_marketplaces)
