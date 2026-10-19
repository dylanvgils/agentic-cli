# Volume mounts

The root filesystem is read-only, so every path a tool writes to is mounted explicitly. This page lists the mounts agentic adds for each tool, and the [checks](#mount-safety) every mount passes. For your own mounts, see [Named volumes](usage.md#named-volumes).

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

A [secret](usage.md#secrets) at `/run/secrets/copilot_token` is exported as `GITHUB_TOKEN` before the CLI starts. To use a token without the agent reading it, use [credential injection](credential-injection.md) with the `github` preset instead.

## OpenCode

OpenCode follows the [XDG spec](https://specifications.freedesktop.org/basedir-spec/latest/) and writes to five directories, so each one is mounted separately.

| Type | Host path                             | Container path                          | Purpose                       |
| ---- | ------------------------------------- | --------------------------------------- | ----------------------------- |
| Bind | `$AGENTIC_HOME/tools/opencode/data`   | `$CONTAINER_HOME/.opencode`             | Main application data         |
| Bind | `$AGENTIC_HOME/tools/opencode/share`  | `$CONTAINER_HOME/.local/share/opencode` | XDG data dir                  |
| Bind | `$AGENTIC_HOME/tools/opencode/state`  | `$CONTAINER_HOME/.local/state/opencode` | XDG state dir (logs, history) |
| Bind | `$AGENTIC_HOME/tools/opencode/cache`  | `$CONTAINER_HOME/.cache/opencode`       | XDG cache dir                 |
| Bind | `$AGENTIC_HOME/tools/opencode/config` | `$CONTAINER_HOME/.config/opencode`      | XDG config dir                |

## Mount safety

Before a run starts, again right before the sidecars start, and once more right before the tool starts (after the sidecars are up or the microsandbox image is loaded), agentic checks every bind mount and secret:

- A mount that would expose `$AGENTIC_HOME/agentic.json` or `state.json` (the agentic home or a parent, such as `~`) is refused, since they hold machine-wide settings, trusted directories and approvals.
- A path through a symlink inside the workspace is refused, so a planted `.git -> ~/.ssh` can't be mounted via `mounts = ["$PWD/.git:/workspace/.git"]`. Mount the real path instead.
- A path under `/proc`, or one agentic can't resolve, is refused.

Not covered:

- A path swapped for a symlink in the moment between a check and Docker or msb mounting it, such as while the proxy starts before the DinD sidecar mounts the workspace. Only something already running in the workspace can do this, such as an agent from another run in the same project.
- A symlink planted inside another `:rw` mount.
