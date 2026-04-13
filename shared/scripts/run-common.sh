# run-common.sh - sourced by tool run scripts, do not execute directly.

# --- Checks ---
# Warn if not in a git repo, this means that the user is
# likely running from the wrong directory.
check_git_repo() {
  if ! git -C "$PWD" rev-parse --git-dir >/dev/null 2>&1; then
    echo "Warning: $PWD is not a git repository"
    read -r -p "Continue anyway? [y/N] " confirm
    [[ "$confirm" =~ ^[yY]$ ]] || exit 1
  fi
}

# Verify the tool image exists. Must be called after IMAGE is set.
check_image() {
  if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
    echo "Error: image '$IMAGE' not found. Build it first with: agentic build"
    exit 1
  fi
}

# Resolve the container home directory from the image's TOOL_HOME env var.
# Must be called after IMAGE is set.
resolve_container_home() {
  CONTAINER_HOME=$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$IMAGE" | grep '^TOOL_HOME=' | cut -d= -f2)
  if [[ -z "$CONTAINER_HOME" ]]; then
    echo "Error: TOOL_HOME is not set in image '$IMAGE'"
    exit 1
  fi
}

# --- Defaults ---
# Base tmpfs mounts. Callers can append to TMPFS_FLAGS after sourcing.
# Set TMPFS_EXEC_TMP=1 before sourcing this file to enable exec on /tmp.
TMPFS_FLAGS=(--tmpfs "/tmp:${TMPFS_EXEC_TMP:+exec,}size=1g")

# Volume mounts. Pre-populated from AGENTIC_EXTRA_MOUNTS (comma-separated
# host:container specs). Callers must append (+=) tool-specific mounts before run_container.
MOUNTS=()
if [[ -n "${AGENTIC_EXTRA_MOUNTS:-}" ]]; then
  IFS=',' read -ra _extra_mounts <<<"$AGENTIC_EXTRA_MOUNTS"
  for _mount in "${_extra_mounts[@]}"; do
    [[ -n "$_mount" ]] && MOUNTS+=(-v "$_mount")
  done
fi

# Base Docker arguments shared across all tools.
DOCKER_ARGS=(
  # Run container in interactive mode, delete when done
  "run" "--rm" "-it"
  # Limit the number of PIDs (processes) the container can spawn
  "--pids-limit=${AGENTIC_PIDS_LIMIT}"
  # Maximum number CPUs the container can utilize
  "--cpus=${AGENTIC_CPUS}"
  # Maximum memory that can be used by the container
  "--memory=${AGENTIC_MEMORY}"
  # Read-only file system
  "--read-only"
  # Security: drop all capabilities
  "--cap-drop=ALL"
  # Security: prevent privilege escalation
  "--security-opt=no-new-privileges:true"
  # Use system user to prevent permission issues on mounted files
  "--user" "$(id -u):$(id -g)"
)

# --- Run ---
# Expand mount placeholders in MOUNTS entries:
#   $TOOL_HOME / ${TOOL_HOME}           - host-side agentic data dir (left of :)
#   $CONTAINER_HOME / ${CONTAINER_HOME} - container home dir (right of :)
# Called automatically by run_container after CONTAINER_HOME is set.
expand_mount_vars() {
  local next_is_mount=0 mount expanded
  local new_mounts=()
  for mount in "${MOUNTS[@]}"; do
    if [[ "$mount" == "-v" ]]; then
      new_mounts+=("-v")
      next_is_mount=1
    elif [[ "$next_is_mount" -eq 1 ]]; then
      expanded="${mount//\$\{CONTAINER_HOME\}/$CONTAINER_HOME}"
      expanded="${expanded//\$CONTAINER_HOME/$CONTAINER_HOME}"
      expanded="${expanded//\$\{TOOL_HOME\}/$TOOL_HOME}"
      expanded="${expanded//\$TOOL_HOME/$TOOL_HOME}"
      new_mounts+=("$expanded")
      next_is_mount=0
    else
      new_mounts+=("$mount")
    fi
  done
  MOUNTS=("${new_mounts[@]}")
}

# Run the container. Expects IMAGE, CONTAINER_HOME, TMPFS_FLAGS, and MOUNTS to be set.
run_container() {
  if [[ -z "${IMAGE:-}" ]]; then
    echo "Error: IMAGE is not set"
    exit 1
  fi
  if [[ -z "${CONTAINER_HOME:-}" ]]; then
    echo "Error: CONTAINER_HOME is not set (call resolve_container_home first)"
    exit 1
  fi

  expand_mount_vars
  docker "${DOCKER_ARGS[@]}" "${TMPFS_FLAGS[@]}" "${MOUNTS[@]}" "$IMAGE" "$@"
}
