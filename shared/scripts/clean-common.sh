# clean-common.sh - sourced by tool clean scripts, do not execute directly.
# Requires: IMAGE (set via config.sh)

clean_tool() {
  docker ps -aq --filter "label=project=agentic-cli" --filter "ancestor=$IMAGE" |
    xargs -r docker rm -f

  docker images -q "$IMAGE" |
    xargs -r docker rmi -f
}
