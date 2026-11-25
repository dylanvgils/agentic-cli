# Register AGENTIC_MARKETPLACES into Copilot
# Not idempotent, so skip marketplaces already registered (e.g. from a previous run against
# the same persisted $HOME/.copilot mount). Matched by local path rather than name, since
# copilot's registered name is derived from the manifest and may differ from ours - see the
# fragility note below.
marketplaces_dir="$HOME/{{.MarketplacesDir}}"
if [[ -n "${AGENTIC_MARKETPLACES:-}" ]]; then
  registered_locs="$(copilot plugin marketplace list 2>/dev/null | awk '{i=index($0,"(Local: "); if (!i) next; loc=substr($0,i+8); sub(/\)$/,"",loc); print loc}')"
  IFS=',' read -ra marketplace_names <<< "$AGENTIC_MARKETPLACES"
  for name in "${marketplace_names[@]}"; do
    dir="$marketplaces_dir/$name"
    [[ -d "$dir" ]] || continue
    grep -qxF "$dir" <<< "$registered_locs" && continue
    copilot plugin marketplace add "$dir" || echo "warning: failed to register marketplace $name" >&2
  done
fi

# Deregister marketplaces this container no longer mounts, e.g. removed from .agenticrc.toml
# copilot plugin marketplace list has no --json output, so parse its "NAME (Local: PATH)" lines instead.
# Fragile: this text format is not a stable contract. Simplify (or drop the path-matching) once
# copilot ships machine-readable list output.
while IFS=$'\t' read -r name loc; do
  [[ -n "$name" && "$(dirname -- "$loc")" == "$marketplaces_dir" && ! -d "$loc" ]] || continue
  copilot plugin marketplace remove --force "$name" || echo "warning: failed to deregister marketplace $name" >&2
done < <(copilot plugin marketplace list 2>/dev/null | awk '{i=index($0,"(Local: "); if (!i) next; name=$0; sub(/ \(Local:.*/,"",name); sub(/^.*[ \t]/,"",name); loc=substr($0,i+8); sub(/\)$/,"",loc); printf "%s\t%s\n",name,loc}')
