# Volume Mounts

Each tool container runs with a read-only filesystem. Any path a tool needs to write to must be explicitly mounted - either as a bind mount from the host, a named Docker volume for persistent state, or a tmpfs for ephemeral scratch space.

This page documents the mounts agentic sets up automatically for each tool. For user-configurable extra mounts (`-v`, `AGENTIC_EXTRA_MOUNTS`, `.agenticrc`), see the [Named Docker volumes](../README.md#-named-docker-volumes) section in the README.

## Common mounts (all tools)

All tools share one volume mount and one tmpfs:

| Type   | Host path | Container path      | Purpose                                                           |
| ------ | --------- | ------------------- | ----------------------------------------------------------------- |
| Volume | `$PWD`    | `/workspace`        | Your working directory - the repo or project the tool operates on |
| Tmpfs  | -         | `/tmp` (1 GB, exec) | Ephemeral scratch space                                           |

## Claude

Claude Code stores session history, project memory, and credentials in two locations under `$AGENTIC_HOME/claude/`.

| Type   | Host path                           | Container path                 | Purpose                                          |
| ------ | ----------------------------------- | ------------------------------ | ------------------------------------------------ |
| Volume | `$AGENTIC_HOME/claude/data`         | `$CONTAINER_HOME/.claude`      | Session history, project memory, and tool config |
| Volume | `$AGENTIC_HOME/claude/.claude.json` | `$CONTAINER_HOME/.claude.json` | Authentication credentials                       |

`.claude.json` is pre-created as an empty `{}` on first run. Claude Code expects this file to exist before it can write credentials - without it, the first login attempt would fail against the read-only root filesystem.

## Copilot

GitHub Copilot CLI persists its auth tokens under `$AGENTIC_HOME/copilot/`.

| Type   | Host path               | Container path                        | Purpose                            |
| ------ | ----------------------- | ------------------------------------- | ---------------------------------- |
| Volume | `$AGENTIC_HOME/copilot` | `$CONTAINER_HOME/.copilot`            | Auth tokens and Copilot CLI config |
| Tmpfs  | -                       | `$CONTAINER_HOME/.cache` (1 GB, exec) | Ephemeral cache                    |

The extra `~/.cache` tmpfs is required because Copilot writes cache data to `~/.cache` rather than `/tmp`. Since the root filesystem is read-only, this path needs its own writable tmpfs.

Copilot also supports secret injection via `--secret`: if a file is mounted at `/run/secrets/copilot_token`, the entrypoint automatically exports it as `GITHUB_TOKEN` before starting the CLI. See [Secrets](../README.md#-secrets) in the README.

## OpenCode

OpenCode follows the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/latest/), splitting its state across five distinct directories.

| Type   | Host path                       | Container path                          | Purpose                       |
| ------ | ------------------------------- | --------------------------------------- | ----------------------------- |
| Volume | `$AGENTIC_HOME/opencode/data`   | `$CONTAINER_HOME/.opencode`             | Main application data         |
| Volume | `$AGENTIC_HOME/opencode/share`  | `$CONTAINER_HOME/.local/share/opencode` | XDG data dir                  |
| Volume | `$AGENTIC_HOME/opencode/state`  | `$CONTAINER_HOME/.local/state/opencode` | XDG state dir (logs, history) |
| Volume | `$AGENTIC_HOME/opencode/cache`  | `$CONTAINER_HOME/.cache/opencode`       | XDG cache dir                 |
| Volume | `$AGENTIC_HOME/opencode/config` | `$CONTAINER_HOME/.config/opencode`      | XDG config dir                |

Each directory serves a distinct purpose under the XDG spec and OpenCode writes to all five, so all five must be separately mounted. Merging them into a single volume would expose unrelated state across the boundaries XDG is designed to separate.
