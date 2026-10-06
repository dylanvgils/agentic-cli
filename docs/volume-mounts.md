# Volume mounts

The root filesystem is read-only, so every path a tool writes to is mounted explicitly. This page lists the mounts agentic adds for each tool. For your own mounts, see [Named Docker volumes](usage.md#named-docker-volumes).

## All tools

| Type  | Host path | Container path      | Purpose                    |
| ----- | --------- | ------------------- | -------------------------- |
| Bind  | `$PWD`    | `/workspace`        | The project you work on    |
| Tmpfs | -         | `/tmp` (1 GB, exec) | Scratch space              |

## Claude

| Type | Host path                                 | Container path                 | Purpose                                 |
| ---- | ----------------------------------------- | ------------------------------ | --------------------------------------- |
| Bind | `$AGENTIC_HOME/tools/claude/data`         | `$CONTAINER_HOME/.claude`      | Session history, memory and tool config |
| Bind | `$AGENTIC_HOME/tools/claude/.claude.json` | `$CONTAINER_HOME/.claude.json` | Login credentials                       |

`.claude.json` is created as `{}` on first run, because Claude Code can't create it on a read-only filesystem.

## Copilot

| Type  | Host path                     | Container path                        | Purpose                     |
| ----- | ----------------------------- | ------------------------------------- | --------------------------- |
| Bind  | `$AGENTIC_HOME/tools/copilot` | `$CONTAINER_HOME/.copilot`            | Auth tokens and CLI config  |
| Tmpfs | -                             | `$CONTAINER_HOME/.cache` (1 GB, exec) | Cache (Copilot ignores `/tmp`) |

A [secret](usage.md#secrets) at `/run/secrets/copilot_token` (`-s copilot_token:<file>`) is exported as `GITHUB_TOKEN` before the CLI starts.

## OpenCode

OpenCode follows the [XDG spec](https://specifications.freedesktop.org/basedir-spec/latest/) and writes to five directories, so each one is mounted separately.

| Type | Host path                             | Container path                          | Purpose                       |
| ---- | ------------------------------------- | --------------------------------------- | ----------------------------- |
| Bind | `$AGENTIC_HOME/tools/opencode/data`   | `$CONTAINER_HOME/.opencode`             | Main application data         |
| Bind | `$AGENTIC_HOME/tools/opencode/share`  | `$CONTAINER_HOME/.local/share/opencode` | XDG data dir                  |
| Bind | `$AGENTIC_HOME/tools/opencode/state`  | `$CONTAINER_HOME/.local/state/opencode` | XDG state dir (logs, history) |
| Bind | `$AGENTIC_HOME/tools/opencode/cache`  | `$CONTAINER_HOME/.cache/opencode`       | XDG cache dir                 |
| Bind | `$AGENTIC_HOME/tools/opencode/config` | `$CONTAINER_HOME/.config/opencode`      | XDG config dir                |
