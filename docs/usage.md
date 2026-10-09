# Usage

```bash
agentic <command> [args...]
```

Run tools from inside a git repository. The current directory is mounted as `/workspace` in the container.

## Commands

Run `agentic help <command>` for all flags.

| Command                                    | Description                                                                          |
| ------------------------------------------ | ------------------------------------------------------------------------------------ |
| `run <tool> [args...]`                     | Run a tool in an isolated container                                                  |
| `run <tool> -- <cmd> [args]`               | Run a command (e.g. `bash`) instead of the tool                                      |
| `build [tool]`                             | Build tool image(s), all tools by default. See [Images](images.md).                  |
| `update [tool]`                            | Rebuild image(s) with the latest tool version                                        |
| `clean [tool]`                             | Remove tool image(s). With no tool, removes everything agentic created.              |
| `inspect [tool]`                           | Show built images, or full detail for one tool                                       |
| `instructions <tool>`                      | Preview the [environment instructions](config.md#runinstructions)                    |
| `status`                                   | Show whether Docker is running and list running agentic containers                   |
| `config`                                   | Show the merged [configuration](config.md)                                           |
| `namespaces <list\|prune>`                 | List namespaces, or remove all images in one                                         |
| `volumes <create\|list\|remove> [name]`    | Manage [named volumes](#named-docker-volumes)                                        |
| `marketplaces <list\|prune>`               | Manage [marketplace](#marketplaces) clones                                           |
| `proxy <build\|update\|clean>`             | Manage the [egress proxy](egress-proxy.md) image                                     |
| `migrate`                                  | Migrate `$AGENTIC_HOME`'s on-disk layout (runs automatically)                        |
| `upgrade`                                  | Upgrade agentic to the latest release                                                |
| `version`                                  | Show version information                                                             |
| `completion <bash\|zsh\|fish\|powershell>` | Print a [shell completion](#shell-completion) script                                 |
| `aliases`                                  | Print [shell aliases](#shell-aliases) for installed tools                            |

Tools: `claude` (Claude Code), `copilot` (GitHub Copilot CLI), `opencode` (OpenCode).

## Examples

```bash
agentic run claude                   # run a tool
agentic run claude -- bash           # open a shell in the tool container
agentic update                       # update all tools (rebuilds only when something changed)

agentic inspect                      # all images in the active namespace
agentic inspect claude --all         # claude images in every namespace

# A separate image set for one project (images are named <namespace>-<tool>)
agentic build claude --namespace myproject --base node,java --apt make
agentic run --namespace myproject claude
agentic update --all                 # update every namespace
```

## Shell completion

Add one line to your shell config:

```bash
source <(agentic completion zsh)                               # ~/.zshrc
source <(agentic completion bash)                              # ~/.bashrc
agentic completion fish | source                               # ~/.config/fish/config.fish
agentic completion powershell | Out-String | Invoke-Expression # $PROFILE
```

Tool names are looked up when you press tab, so you don't need to regenerate the script for new tools.

## Shell aliases

Aliases let you type `copilot` instead of `agentic run copilot`. The shell is detected automatically. Only tools with a built image get an alias.

```bash
source <(agentic aliases)                               # bash/zsh
agentic aliases | source                                # fish
agentic aliases | Out-String | Invoke-Expression        # PowerShell
```

## Secrets

`-s`/`--secret` mounts a file read-only, using the format `name:/path/to/file[:/container/path]`. Without a container path, the file goes to `/run/secrets/<name>`. Paths support `~`, `$HOME` and (on the container side) `$CONTAINER_HOME`.

```bash
agentic run -s 'copilot_token:~/.secrets/copilot_token' copilot
agentic run -s 'maven-settings:~/.m2/settings.xml:$CONTAINER_HOME/.m2/settings.xml' claude
```

The agent can read mounted secrets. For API keys, use [credential injection](egress-proxy.md#credential-injection) so the agent only sees a placeholder.

## Environment variables

`-e`/`--env` sets `KEY=VALUE`, or forwards the host value with a bare `KEY` (skipped if unset):

```bash
agentic run -e NODE_OPTIONS=--max-old-space-size=4096 claude
agentic run -e CI claude
```

- Values are visible inside the container and in `docker inspect`. Use `--secret` for tokens.
- Names agentic manages are refused: `TOOL_HOME`, `CONTAINER_HOME`, `AGENTIC_MARKETPLACES` and `AGENTIC_PROXY_CA` always; the proxy vars with `--proxy`; the `DOCKER_*` vars with [`--dind`](docker-in-docker.md#how-it-stays-isolated); the CA and credential vars with [credential injection](egress-proxy.md#credential-injection).
- `TZ` and the terminal vars (`COLORTERM`, `TERM`, `NO_COLOR`, `FORCE_COLOR`) are forwarded from the host automatically. Override them like any other var (`-e TZ=UTC`).

## Named Docker volumes

`-v` takes a bind mount (`host/path:container/path`) or a named volume (`name:container/path`). Named volumes are created on first use and persist across runs. Paths support [placeholders](config.md#mount-variable-expansion).

```bash
agentic run -v 'maven:$CONTAINER_HOME/.m2' claude    # named volume, read-write
agentic run -v '~/notes:/notes' claude               # bind mount, read-only
agentic run -v '~/.m2:$CONTAINER_HOME/.m2:rw' claude # bind mount, writable
```

- Bind mounts are **read-only unless you add `:rw`**, so the agent can't change host files you only meant to share. Other Docker options can follow it (`:rw,z`).
- Named volumes, `/workspace` and the tool's own state directories are always writable.
- See [Volume mounts](volume-mounts.md) for what each tool gets mounted automatically.

Manage volumes with `agentic volumes`:

```bash
agentic volumes create maven
agentic volumes list              # alias: ls
agentic volumes remove maven      # alias: rm
agentic volumes remove            # remove all agentic volumes
```

## Docker context

`--docker-context` picks a [Docker context](https://docs.docker.com/engine/manage-resources/contexts/) other than the active one, and tab-completes from `docker context ls`. `agentic status` shows the context in use when it isn't the default. Set a default with [`docker_context`](config.md#top-level-keys).

```bash
agentic --docker-context prod build claude
```

## Data directory

`$AGENTIC_HOME` (default `~/.agentic`) holds:

- `tools/<tool>/` - each tool's state. See [Volume mounts](volume-mounts.md) for the paths.
- `marketplaces/` - [marketplace](#marketplaces) clones
- `logs/` - agentic's own logs, e.g. `proxy_<id>.jsonl`
- `custom-installs/` - the [custom installs](config.md#buildcustom_installs) each image was built with, so `agentic update` can rebuild them. `agentic clean` removes the ones no image uses anymore.

`--home` overrides it for one command, and every command accepts it: `agentic --home /opt/agentic run claude`.

## Marketplaces

[`[[marketplaces]]`](config.md#marketplaces) entries are git plugin repos (skills, agents, commands, hooks, MCP servers) shared with tool containers.

- Before each run, agentic clones or updates each marketplace on the host (`git fetch` + `git reset --hard`). It uses your own git auth, so no credentials enter the container. `git` must be on your `PATH`.
- A failed first clone fails the run. A failed update only warns and reuses the existing clone.
- Clones live in `$AGENTIC_HOME/marketplaces/<slug>-<hash>/`, keyed by `url`. Projects that use the same URL share one clone, even under different names. Usage is tracked in `.usage.json` there.
- Each clone is mounted read-only at `~/marketplaces/<name>` and registered with the tool. Removing an entry unregisters it on the next run. Copilot only warns when registration fails.

```bash
agentic marketplaces list    # one row per clone and name, with the projects using it (alias: ls)
agentic marketplaces prune   # remove clones no project uses anymore
```

`prune` checks each project's current config first, and keeps a clone while any of its names is still in use. It leaves alone clones that agentic didn't create (e.g. ones you placed by hand). `agentic clean` doesn't touch clones.
