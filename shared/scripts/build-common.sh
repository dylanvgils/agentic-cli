# build-common.sh - sourced by tool build scripts, do not execute directly.
# Requires: BASE, IMAGE (set via config.sh)
# Optional env: AGENTIC_BASE_OVERRIDE   (set by bin/agentic --base flag)
#               AGENTIC_NO_CACHE        (set by bin/agentic --no-cache flag)
#               AGENTIC_NO_CACHE_TOOL   (set by update_tool; skips cache on tool step only)
#               AGENTIC_NODE_VERSION    (set by bin/agentic --node flag)
#               AGENTIC_<EXTRA>_VERSION (set by bin/agentic --<extra> flag, e.g. AGENTIC_JAVA_VERSION)

build_tool() {
  local tool_dir="$1"
  local extras="${AGENTIC_BASE_OVERRIDE:-$BASE}"
  local cache_flag="${AGENTIC_NO_CACHE:+--no-cache}"
  # AGENTIC_NO_CACHE_TOOL skips cache only on the tool image step (Steps 3-4).
  # AGENTIC_NO_CACHE overrides this and applies to all steps.
  local tool_cache_flag
  tool_cache_flag="${cache_flag:-${AGENTIC_NO_CACHE_TOOL:+--no-cache}}"

  # Step 1: Build the node root layer
  docker build $cache_flag \
    ${AGENTIC_NODE_VERSION:+--build-arg NODE_VERSION=$AGENTIC_NODE_VERSION} \
    -t agentic-base \
    "$REPO_ROOT/shared/base/node"

  local node_ver
  node_ver=$(docker run --rm agentic-base node --version 2>/dev/null | tr -d '\r' | grep -oE '[0-9]+(\.[0-9]+)*' | head -1 || true)

  # Step 2: Layer any extras on top of node, left to right
  local prev_image="agentic-base"
  local tag_suffix=""
  local base_label="node${node_ver:+@$node_ver}"

  if [[ -n "$extras" ]]; then
    IFS=',' read -ra extra_list <<<"$extras"

    for extra in "${extra_list[@]}"; do
      local base_dir="$REPO_ROOT/shared/base/$extra"

      if [[ ! -d "$base_dir" ]]; then
        echo "Error: unknown base '$extra' (no directory at shared/base/$extra)" >&2
        exit 1
      fi

      tag_suffix="${tag_suffix:+$tag_suffix-}$extra"
      local image_tag="agentic-base-$tag_suffix"

      # Derive the version env var for this extra: e.g. "java" → AGENTIC_JAVA_VERSION
      local version_var="AGENTIC_$(echo "$extra" | tr '[:lower:]' '[:upper:]')_VERSION"

      docker build $cache_flag \
        --build-arg BASE_IMAGE="$prev_image" \
        ${!version_var:+--build-arg $(echo "$extra" | tr '[:lower:]' '[:upper:]')_VERSION=${!version_var}} \
        -t "$image_tag" \
        "$base_dir"

      # Always detect the actual installed version from the running container so the
      # label reflects reality (e.g. "25.0.1") rather than the build-arg ("25").
      # grep -oE extracts the first semver-like token, which works across tools:
      #   openjdk 25.0.1 2025-09-16 LTS   →  25.0.1
      #   node v24.2.0                    →  24.2.0
      #   git version 2.47.1              →  2.47.1
      local extra_ver
      extra_ver=$(docker run --rm "$image_tag" sh -c "$(echo "$extra") --version" 2>/dev/null |
        head -1 | tr -d '\r' | grep -oE '[0-9]+(\.[0-9]+)*' | head -1 || true)
      base_label="${base_label},$extra${extra_ver:+@$extra_ver}"

      prev_image="$image_tag"
    done
  fi

  # Step 3: Build the tool image using the final composed base
  docker build $tool_cache_flag \
    --build-arg HOST_UID="$(id -u)" \
    --build-arg HOST_GID="$(id -g)" \
    --build-arg BASE_IMAGE="$prev_image" \
    --label "agentic.base=$base_label" \
    --label "agentic.built=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -t "$IMAGE" \
    "$tool_dir"

  # Step 4: Capture installed tool version as a label (requires VERSION_CMD from config.sh)
  if [[ -n "${VERSION_CMD:-}" ]]; then
    local tool_version
    tool_version=$(docker run --rm --entrypoint="" "$IMAGE" sh -c "$VERSION_CMD" 2>/dev/null | head -1 | tr -d '\r' || true)
    if [[ -n "$tool_version" ]]; then
      docker build \
        --label "agentic.tool.version=$tool_version" \
        -t "$IMAGE" - <<EOF
FROM $IMAGE
EOF
    fi
  fi
}
