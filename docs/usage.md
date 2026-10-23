# Usage

```bash
agentic <command> [args...]
```

Run tools from your project directory. It's mounted as `/workspace` in the container.

agentic's flags go before the tool name. Everything after it goes to the tool:

```bash
agentic run --proxy claude --resume   # --proxy for agentic, --resume for Claude
```

## First run

1. Build the tool's image once:

   ```bash
   agentic build claude
   ```

2. Start it from your project directory:

   ```bash
   cd ~/projects/my-app
   agentic run claude
   ```

3. Answer `y` to `trust directory ...?`. agentic mounts a trusted directory and its subdirectories writable and reads their `.agenticrc.toml`, so trust only your own projects. Non-interactive runs need `--trust-dir`. To revoke trust, see [`state.json`](config.md#statejson).
4. Sign in the way the tool asks. The container has no browser, so open any link it prints on your machine.

   | Tool     | Sign in                                                    |
   | -------- | ---------------------------------------------------------- |
   | claude   | Follow the login prompt on first start                     |
   | copilot  | Type `/login` in the tool                                  |
   | opencode | `agentic run opencode auth login`                          |

The login is saved in the tool's home under `$AGENTIC_HOME/tools/<tool>`, so it survives runs and is shared by all your projects. The agent can read it. To keep an API key out of its reach instead, use [credential injection](credential-injection.md).

## Commands

Run `agentic help <command>` for all flags.

| Command                                    | Description                                                                          |
| ------------------------------------------ | ------------------------------------------------------------------------------------ |
| `run <tool> [args...]`                     | Run a tool in an isolated container                                                  |
| `run <tool> -- <cmd> [args]`               | Run a command (e.g. `bash`) instead of the tool                                      |
| `build [tool]`                             | Build tool image(s), all tools by default. See [Images](images.md).                  |
| `update [tool]`                            | Rebuild image(s) with the latest tool version                                        |
| `clean [tool]`                             | Remove tool image(s) and their msb copies, or everything agentic created. Skips what running tools use. |
| `inspect [tool]`                           | Show built images, or full detail for one tool                                       |
| `instructions <tool>`                      | Preview the [environment instructions](environment-instructions.md)                    |
| `ps`                                       | List running agentic containers and VMs in the active namespace, `--all` for every namespace |
| `status`                                   | Show Docker and microsandbox availability, and how many agentic containers and VMs run |
| `config`                                   | Show the merged [configuration](config.md)                                           |
| `namespaces <list\|prune>`                 | List namespaces, or remove all images in one                                         |
| `volumes <create\|list\|remove> [name]`    | Manage [named volumes](#named-volumes)                                               |
| `marketplaces <list\|prune>`               | Manage [marketplace](#marketplaces) clones                                           |
| `proxy <build\|update\|clean>`             | Manage the [egress proxy](egress-proxy.md) image                                     |
| `upgrade`                                  | Upgrade agentic to the latest release                                                |
| `version`                                  | Show version information, including the Docker CLI and microsandbox versions         |
| `completion <bash\|zsh\|fish\|powershell>` | Print a [shell completion](#shell-completion) script                                 |
| `aliases`                                  | Print [shell aliases](#shell-aliases) for installed tools                            |

Tools: `claude` (Claude Code), `copilot` (GitHub Copilot CLI), `opencode` (OpenCode).

## Examples

```bash
agentic run claude                         # run a tool
agentic run claude -- bash                 # open a shell in the tool container
agentic run --runtime microsandbox claude  # run in a microVM instead
agentic run --dry-run --proxy claude      # print the docker or msb commands, start nothing
agentic update                             # update all tools (rebuilds only when something changed)

agentic inspect                            # all images in the active namespace
agentic inspect claude --all               # claude images in every namespace

# A separate image set for one project (images are named <namespace>-<tool>)
agentic build claude --namespace myproject --base node,java --apt make
agentic run --namespace myproject claude
agentic update --all                       # update every namespace
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

Aliases let you type `copilot` instead of `agentic run copilot`. The shell is detected automatically. Only tools with a built image get an alias. Arguments after an alias go to the tool, so use `agentic run` when you need agentic's flags.

The output also defines `agt` as shorthand for `agentic`, for example `agt build claude`. For tab completion on `agt`, put the aliases line after the [shell completion](#shell-completion) line in your shell config.

```bash
source <(agentic aliases)                               # bash/zsh
agentic aliases | source                                # fish
agentic aliases | Out-String | Invoke-Expression        # PowerShell
```

## Environment variables

`-e`/`--env` sets `KEY=VALUE`, or forwards the host value with a bare `KEY` (skipped if unset):

```bash
agentic run -e NODE_OPTIONS=--max-old-space-size=4096 claude
agentic run -e CI claude
```

- Values are visible inside the container and in `docker inspect`. Use [credential injection](credential-injection.md) for API keys.
- Names agentic manages are refused: `TOOL_HOME`, `CONTAINER_HOME`, `AGENTIC_MARKETPLACES` and `AGENTIC_PROXY_CA` always; the proxy vars with `--proxy`; the `DOCKER_*` vars with [`--dind`](docker-in-docker.md#how-it-stays-isolated); the CA and credential vars with [credential injection](credential-injection.md).
- `TZ` and the terminal vars (`COLORTERM`, `TERM`, `NO_COLOR`, `FORCE_COLOR`) are forwarded from the host automatically. Override them like any other var (`-e TZ=UTC`).

## Named volumes

`--volume` takes a bind mount (`host/path:container/path`) or a named volume (`name:container/path`). Named volumes are created on first use and persist across runs. Paths support [placeholders](config.md#mount-variable-expansion).

```bash
agentic run -v 'maven:$CONTAINER_HOME/.m2' claude    # named volume, read-write
agentic run -v '~/notes:/notes' claude               # bind mount, read-only
agentic run -v '~/.m2:$CONTAINER_HOME/.m2:rw' claude # bind mount, writable
```

- Bind mounts are **read-only unless you add `:rw`**, so the agent can't change host files you only meant to share. Other Docker options can follow it (`:rw,z`).
- Named volumes, `/workspace` and the tool's own state directories are always writable.
- See [Volume mounts](volume-mounts.md) for what each tool gets mounted automatically.

To protect part of the workspace, mount it over itself. It stays read-only while the rest of `/workspace` is writable:

```bash
agentic run -v '$PWD/.git:/workspace/.git' claude
```

Manage volumes with `agentic volumes`:

```bash
agentic volumes create maven
agentic volumes list              # alias: ls
agentic volumes remove maven      # alias: rm
agentic volumes remove            # remove all agentic volumes
```

With the microsandbox runtime, msb stores the volume as `agentic-<name>`, so agentic can tell its volumes from yours. `list` and `remove` cover these too; `create` is Docker-only, since msb creates volumes on first use.

## Docker context

`--docker-context` picks a [Docker context](https://docs.docker.com/engine/manage-resources/contexts/) other than the active one, and tab-completes from `docker context ls`. `agentic status` shows the context in use. Set a default with [`docker_context`](config.md#top-level-keys).

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

- Before each run, agentic clones or updates each marketplace on the host (`git fetch` + `git reset --hard`). It uses your own git auth, so no credentials enter the container. `git` must be on your `PATH`, and only https and ssh are allowed, so a marketplace can't expose a host path.
- A failed first clone fails the run. A failed update only warns and reuses the existing clone.
- Clones live in `$AGENTIC_HOME/marketplaces/<slug>-<hash>/`, keyed by `url`. Projects that use the same URL share one clone, even under different names.
- Each clone is mounted read-only at `~/marketplaces/<name>` and registered with the tool. Removing an entry unregisters it on the next run. Copilot only warns when registration fails.

```bash
agentic marketplaces list    # one row per clone and name, with the projects using it (alias: ls)
agentic marketplaces prune   # remove clones no project uses anymore
```

`prune` checks each project's current config first, and keeps a clone while any of its names is still in use. It leaves alone clones that agentic didn't create (e.g. ones you placed by hand). `agentic clean` doesn't touch clones.
