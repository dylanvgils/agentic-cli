# Usage

```bash
agentic <command> [args...]
```

Run tool commands from within a git repository. The current directory is mounted as `/workspace` inside the container.

## Commands

Run `agentic help <command>` (or `agentic <command> --help`) for the full list of flags.

| Command                                       | Description                                                                                        |
| --------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| `run <tool> [args...]`                        | Run a tool in an isolated Docker container                                                         |
| `run <tool> -- <cmd> [args]`                  | Override the entrypoint and run a shell command directly                                           |
| `build [tool]`                                | Build tool image(s). Builds all tools if unspecified                                               |
| `update [tool]`                               | Update tool image(s) to latest version                                                             |
| `clean [tool]`                                | Remove tool image(s). No-arg form removes everything agentic created                               |
| `inspect [tool]`                              | Show built images, or full detail for one tool                                                     |
| `instructions <tool>`                         | Preview the [environment instructions](#environment-instructions) without starting a container    |
| `status`                                      | Show whether the Docker backend is running and list running agentic containers                     |
| `config`                                      | Show the merged configuration from agentic.json and all .agenticrc.toml files                      |
| `namespaces <list\|prune>`                    | List namespaces, or remove all images in one                                                       |
| `volumes <create\|list\|remove> [name]`       | Manage named Docker volumes created by agentic                                                     |
| `marketplaces <list\|prune>`                  | Manage marketplace clones downloaded by `agentic run`                                              |
| `proxy <build\|update\|clean>`                | Manage the egress proxy image                                                                      |
| `migrate`                                     | Apply any pending migrations to `$AGENTIC_HOME`'s on-disk layout (runs automatically)              |
| `upgrade`                                     | Upgrade the agentic binary to the latest release                                                   |
| `version`                                     | Show version information                                                                           |
| `completion <bash\|zsh\|fish\|powershell>`    | Generate shell completion script for the specified shell                                           |
| `aliases`                                     | Print shell alias definitions for installed tools                                                  |

### Tools

| Tool       | Description        |
| ---------- | ------------------ |
| `claude`   | Claude Code CLI    |
| `copilot`  | GitHub Copilot CLI |
| `opencode` | OpenCode CLI       |

## Examples

For building and updating images (runtimes, version pinning, apt packages), see [Images](images.md).

```bash
# Update to latest version (checks upstream first; rebuilds only if newer or base images changed)
agentic update

# Clean / inspect images
agentic clean
agentic clean claude
agentic inspect                      # table of all agentic images in the active namespace
agentic inspect claude               # full detail for active namespace's claude image
agentic inspect claude --all         # detail for all namespaces' claude image

# Build a project-specific image set (images are named <namespace>-<tool>)
agentic build claude --namespace myproject --base node,java --apt make
agentic inspect                      # shows both agentic-claude and myproject-claude
agentic run --namespace myproject claude
agentic update --all                 # update every agentic image across all namespaces

# Run a tool
agentic run claude

# Run a shell command instead of the tool entrypoint
agentic run claude -- bash

# Mount named Docker volumes (auto-created on first use)
agentic run -v 'maven:$CONTAINER_HOME/.m2' -v 'gradle:$CONTAINER_HOME/.gradle' claude

# Mount bind-mount volumes (host paths)
agentic run -v '~/.m2:$CONTAINER_HOME/.m2' claude

# Mount a secret file read-only at /run/secrets/<name>
agentic run -s 'copilot_token:~/.secrets/copilot_token' copilot

# Override the tool home directory
agentic run --home /opt/agentic claude

# Print completion script
agentic completion zsh
```

## Shell completion

Tab completion is available for bash, zsh, fish, and PowerShell. Add one of the following to your shell config to activate it:

```bash
# zsh - add to ~/.zshrc
source <(agentic completion zsh)

# bash - add to ~/.bashrc
source <(agentic completion bash)

# fish - add to ~/.config/fish/config.fish
agentic completion fish | source

# PowerShell - add to your $PROFILE
agentic completion powershell | Out-String | Invoke-Expression
```

Tool names are discovered dynamically at completion time, so new tools are picked up automatically without regenerating the script.

## Shell aliases

Shell aliases let you run tools directly (e.g., `copilot` instead of `agentic run copilot`). The shell is detected automatically. Add to your shell config to activate them:

```bash
# bash/zsh - add to ~/.bashrc or ~/.zshrc
source <(agentic aliases)

# fish - add to ~/.config/fish/config.fish
agentic aliases | source

# PowerShell - add to your $PROFILE
agentic aliases | Out-String | Invoke-Expression
```

Only tools with a built image produce an alias, so sourcing the output never creates broken aliases for uninstalled tools.

## Secrets

Use `--secret` / `-s` to mount a secret file read-only into the container:

```bash
agentic run -s 'copilot_token:~/.secrets/copilot_token' copilot
```

Secrets use the format `name:/path/to/file[:/container/path]`. The `~`, `$HOME`, and `${HOME}` prefixes are expanded to your home directory. Without a container path the file is mounted at `/run/secrets/<name>`; with one it is mounted at the specified path (supports `$CONTAINER_HOME`):

```bash
# Mount Maven settings.xml at the path Maven expects
agentic run -s 'maven-settings:~/.m2/settings.xml:$CONTAINER_HOME/.m2/settings.xml' claude
```

For persisting secrets via `.agenticrc.toml`, see [Configuration](config.md).

A mounted secret is readable by the agent. For an API key the tool only sends as a header (Anthropic, OpenAI, a GitHub token, ...), use [credential injection](config.md#credential-injection) instead: the egress proxy adds the key to requests and the container only sees a placeholder.

## Environment variables

Use `--env` / `-e` to set an environment variable in the container, either as a literal `KEY=VALUE` or a bare `KEY` to forward the host's current value:

```bash
agentic run -e NODE_OPTIONS=--max-old-space-size=4096 claude
agentic run -e CI claude   # forwards the host's CI value, omitted if unset
```

The container's `TZ` is also auto-detected from the host and forwarded automatically, so its clock matches the host instead of defaulting to UTC - override it the same way as any other var (`agentic run -e TZ=UTC claude`).

See [Configuration](config.md) for names agentic already manages that can't be overridden this way, and for persisting variables via `.agenticrc.toml`. Values set with `--env` are visible inside the container and via `docker inspect`/`ps`, so use `--secret` / `-s` for tokens or credentials instead.

## Named Docker volumes

The `-v` flag supports both bind mounts (host paths) and named Docker volumes - named volumes are created automatically on first use and persist across container runs, no host path required. See [Examples](#examples) above for the mount syntax, [Configuration](config.md) for `.agenticrc.toml` persistence, and [Volume mounts](volume-mounts.md) for a per-tool breakdown of what's mounted automatically and why.

### Managing volumes

Use `agentic volumes` to inspect and clean up agentic-managed volumes:

```bash
agentic volumes create maven      # Create a named volume
agentic volumes list              # List all agentic-managed volumes (alias: ls)
agentic volumes remove maven      # Remove a specific volume (alias: rm)
agentic volumes remove            # Remove all agentic-managed volumes
```

## Docker context

If Docker is configured with multiple [contexts](https://docs.docker.com/engine/manage-resources/contexts/), use `--docker-context` (tab-completes against `docker context ls`) to target a specific one instead of whichever is currently active:

```bash
agentic --docker-context prod build claude
```

For persisting a default via `.agenticrc.toml` or `agentic.json`, see [`docker_context`](config.md#docker_context).

## Tool home directory

Each tool stores its configuration under `$AGENTIC_HOME/tools/`:

| Tool       | Config path                                                               |
| ---------- | -------------------------------------------------------------------------- |
| `claude`   | `$AGENTIC_HOME/tools/claude/`, `$AGENTIC_HOME/tools/claude/.claude.json`  |
| `copilot`  | `$AGENTIC_HOME/tools/copilot/`                                            |
| `opencode` | `$AGENTIC_HOME/tools/opencode/` (data, share, state, cache, config)       |

`$AGENTIC_HOME/marketplaces/<slug>-<hash>/` holds host-side clones of any `[[marketplaces]]` configured in `.agenticrc.toml` - shared across tools and mounted read-only into each applicable tool's container. The clone is keyed by the marketplace's `url` alone, so two projects referencing the same URL always share one clone, even under different local names. See [Configuration](config.md) for the full `[[marketplaces]]` reference.

`$AGENTIC_HOME/logs/` holds log files written by agentic's own components, named by type - proxy access logs are written there as `proxy_<id>.jsonl`.

### Managing marketplaces

Marketplaces are downloaded implicitly the first time `agentic run` needs them - no separate install step. Use `agentic marketplaces` to see what's downloaded and clean up clones no longer used:

```bash
agentic marketplaces list    # List synced clones and the name(s)/project(s) referencing each (alias: ls)
agentic marketplaces prune   # Remove clones no project references anymore, under any name
```

`prune` only drops a clone once no known project references it under any name - see [Configuration](config.md) for the exact rules.

## Environment instructions

Every `agentic run` writes a generated block - what's installed, what's restricted, and the network situation - into the tool's own global instructions file (`CLAUDE.md`, `AGENTS.md`, `copilot-instructions.md`), so the model knows the container's constraints up front. Append your own notes via `custom` under `[run.instructions]` in `.agenticrc.toml`, or turn it off with `enabled = false`. See [Configuration](config.md#keys) for the full reference.
