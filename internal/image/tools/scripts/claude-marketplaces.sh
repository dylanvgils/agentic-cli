# Register AGENTIC_MARKETPLACES into the Claude container
marketplaces_dir="$HOME/{{.MarketplacesDir}}"
if [[ -n "${AGENTIC_MARKETPLACES:-}" ]]; then
  IFS=',' read -ra marketplace_names <<< "$AGENTIC_MARKETPLACES"
  for name in "${marketplace_names[@]}"; do
    dir="$marketplaces_dir/$name"
    [[ -d "$dir" ]] || continue
    claude plugin marketplace add "$dir" --scope user || echo "warning: failed to register marketplace $name" >&2
  done
fi

# Deregister marketplaces this container no longer mounts, e.g. removed from .agenticrc.toml
while IFS=$'\t' read -r name loc; do
  [[ -n "$name" && "$(dirname -- "$loc")" == "$marketplaces_dir" && ! -d "$loc" ]] || continue
  claude plugin marketplace remove "$name" --scope user || echo "warning: failed to deregister marketplace $name" >&2
done < <(claude plugin marketplace list --json 2>/dev/null | jq -r '.[] | select(.source=="directory") | [.name, .installLocation] | @tsv' 2>/dev/null)
