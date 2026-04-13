# update-common.sh - sourced by tool update scripts, do not execute directly.
# Requires: BASE, IMAGE (set via config.sh)
# Optional env: AGENTIC_BASE_OVERRIDE, AGENTIC_NODE_VERSION, AGENTIC_<EXTRA>_VERSION
#               (same as build-common.sh; base layers are built with cache by default)
#               Pass --no-cache to bin/agentic update to rebuild all layers.

# shellcheck source=./build-common.sh
source "${REPO_ROOT}/shared/scripts/build-common.sh"

update_tool() {
  local tool_dir="$1"

  # If the user didn't supply --base explicitly, recover the original base from the
  # existing image's agentic.base label so updates don't silently drop extra layers.
  # e.g. label "node@24.2.0,java@21.0.1" → AGENTIC_BASE_OVERRIDE=java
  if [[ -z "${AGENTIC_BASE_OVERRIDE:-}" ]]; then
    local existing_label
    existing_label=$(docker inspect --format '{{index .Config.Labels "agentic.base"}}' "$IMAGE" 2>/dev/null || true)

    if [[ -n "$existing_label" ]]; then
      local recovered_extras=""
      IFS=',' read -ra label_parts <<<"$existing_label"

      for part in "${label_parts[@]}"; do
        local name="${part%%@*}"
        [[ "$name" == "node" ]] && continue
        recovered_extras="${recovered_extras:+$recovered_extras,}$name"
      done

      [[ -n "$recovered_extras" ]] && export AGENTIC_BASE_OVERRIDE="$recovered_extras"
    fi
  fi

  export AGENTIC_NO_CACHE_TOOL=1
  build_tool "$tool_dir"
}
