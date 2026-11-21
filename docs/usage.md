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
| `build <tool\|--all>`                      | Build one tool's image, or all tools' with `--all`. See [Images](images.md).         |
| `update <tool\|--all>`                     | Rebuild image(s) with the latest tool version, `--all` for every namespace           |
| `clean <tool\|--all>`                      | Remove tool image(s) and their msb copies, or everything agentic created with `--all`. Skips what running tools use. |
| `prune`                                    | [Free disk space](#disk-space) without removing tool images                          |
| `image <list\|prune>`, `images`            | List built images, or remove the untagged ones rebuilds left behind                  |
| `inspect <tool>`                           | Show full detail for one tool's image                                                |
| `instructions <tool>`                      | Preview the [environment instructions](environment-instructions.md)                    |
| `ps`                                       | List running agentic containers and VMs in the active namespace, `--all` for every namespace |
| `status`                                   | Show Docker and microsandbox availability, and how many agentic containers and VMs run |
| `config`                                   | Show the merged [configuration](config.md)                                           |
| `namespace <list\|prune>`, `namespaces`    | List namespaces, or remove all images in one                                         |
| `volume <create\|list\|remove>`, `volumes` | Manage [named volumes](#named-volumes)                                        |
| `marketplace <list\|prune>`, `marketplaces` | Manage [marketplace](#marketplaces) clones                                          |
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
agentic update claude                      # update a tool (rebuilds only when something changed)

agentic images                             # all images in the active namespace
agentic inspect claude --all               # claude images in every namespace

# A separate image set for one project (images are named <namespace>-<tool>)
agentic build claude --namespace myproject --base node,java --apt make
agentic run --namespace myproject claude
agentic update --all                       # update every image in every namespace
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
- Names agentic manages are refused: `TOOL_HOME`, `CONTAINER_HOME`, `AGENTIC_MARKETPLACES` and `AGENTIC_PROXY_CA` always; the proxy vars with `--proxy`; the `DOCKER_*` vars with [`--dind`](docker-in-docker.md#how-it-stays-isolated); `MAVEN_ARGS` and `GRADLE_OPTS` with the `maven` and `gradle` [proxy integrations](egress-proxy.md#tools-that-ignore-proxy-env-vars); the CA and credential vars with [credential injection](credential-injection.md).
- `TZ` and the terminal vars (`COLORTERM`, `TERM`, `TERM_PROGRAM`, `TERM_PROGRAM_VERSION`, `NO_COLOR`, `FORCE_COLOR`) are forwarded from the host automatically. Override them like any other var (`-e TZ=UTC`).

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

Manage volumes with `agentic volume`:

```bash
agentic volume create maven
agentic volume list               # alias: ls, or agentic volumes
agentic volume remove maven       # alias: rm
agentic volume remove --all       # remove all agentic volumes
```

With the microsandbox runtime, msb stores the volume as `agentic-<name>`, so agentic can tell its volumes from yours. `list` and `remove` cover these too; `create` is Docker-only, since msb creates volumes on first use.

## Forwarding ports

`--port` forwards a port on your machine to the tool, for example to open a dev server the agent started. It takes `PORT` or `HOST:GUEST`, optionally with `/tcp`, and can be repeated. `[run] ports` in [`.agenticrc.toml`](config.md#run) sets the same list.

```bash
agentic run --port 3000 claude              # localhost:3000 -> 3000 in the tool
agentic run --port 8080:5173 --port 4200 claude
```

- Ports bind to `127.0.0.1` only, so nothing else on your network reaches them. TCP only.
- The server in the tool must listen on `0.0.0.0`, not `localhost`. For Vite, that's `vite --host`.
- The forward lets you into the tool, not the tool out to your machine, but pages you open from it run in your browser. See [Leftover risk](security-model.md#leftover-risk).
- A run refuses a host port that's already in use, and Docker refuses ports on a remote [Docker context](#docker-context).
- Docker can't forward ports with [`--proxy`](egress-proxy.md), as the run network has no route to the host. Pass `--no-proxy` or use the [microsandbox runtime](microsandbox.md), which forwards them either way.
- On Docker, ports that [`--dind`](docker-in-docker.md#how-it-stays-isolated) containers publish stay inside the run. To open one, the agent relays a forwarded port to it; the [environment instructions](environment-instructions.md) tell it how.
- `agentic ps` lists each run's forwarded ports.

## Disk space

After `build`, `update` and `proxy build`, agentic removes the old images that a rebuild left untagged (`agentic image prune` does just that). To free more space, run:

```bash
agentic prune           # asks before clearing the build cache
agentic prune --force   # clears it without asking
agentic prune --logs    # also removes every run's logs
```

`prune` removes sidecars and run networks that interrupted runs left behind, plus untagged agentic images, and keeps what active runs use. It then clears Docker's unused build cache. Docker can't tell agentic's cache apart, so this also clears the cache of your other builds and can slow down the next build.

## Docker context

`--docker-context` picks a [Docker context](https://docs.docker.com/engine/manage-resources/contexts/) other than the active one, and tab-completes from `docker context ls`. `agentic status` shows the context in use. Set a default with [`docker_context`](config.md#top-level-keys).

```bash
agentic --docker-context prod build claude
```

## Data directory

`$AGENTIC_HOME` (default `~/.agentic`) holds:

- `tools/<tool>/` - each tool's state. See [Volume mounts](volume-mounts.md) for the paths.
- `marketplaces/` - [marketplace](#marketplaces) clones
- `logs/<YYYYMMDD>-<HHMMSS>-<id>/` - one dir per run, named after its start time and kept for [`log_retention_days`](config.md#agenticjson), or until [`agentic prune --logs`](#disk-space):
  - `commands.jsonl` - every `docker` and `msb` command agentic ran for the run, as JSON lines with its start time, exit code and duration. `--env` values show as `***`.
  - `proxy.jsonl` - the proxy's [access log](egress-proxy.md#access-log), with the proxy on.
- `custom-installs/` - the [custom installs](config.md#buildcustom_installs) each image was built with, so `agentic update` can rebuild them. `agentic clean` removes the ones no image uses anymore.

`--home` overrides it for one command, and every command accepts it: `agentic --home /opt/agentic run claude`.

## Marketplaces

[`[[marketplaces]]`](config.md#marketplaces) entries are git plugin repos (skills, agents, commands, hooks, MCP servers) shared with tool containers.

- Before each run, agentic clones or updates each marketplace on the host (`git fetch` + `git reset --hard`). It uses your own git auth, so no credentials enter the container. `git` must be on your `PATH`, and only https and ssh are allowed, so a marketplace can't expose a host path.
- A failed first clone fails the run. A failed update only warns and reuses the existing clone.
- Clones live in `$AGENTIC_HOME/marketplaces/<slug>-<hash>/`, keyed by `url`. Projects that use the same URL share one clone, even under different names.
- Each clone is mounted read-only at `~/marketplaces/<name>` and registered with the tool. Removing an entry unregisters it on the next run. Copilot only warns when registration fails.

```bash
agentic marketplace list    # one row per clone and name, with the projects using it (alias: ls, or agentic marketplaces)
agentic marketplace prune   # remove clones no project uses anymore
```

`prune` checks each project's current config first, and keeps a clone while any of its names is still in use. It leaves alone clones that agentic didn't create (e.g. ones you placed by hand). `agentic clean` doesn't touch clones.
