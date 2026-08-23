# Configuration

Agentic is configured through four layers, applied in order of increasing specificity: `agentic.json` global file, `.agenticrc.toml` project files, environment variables, and CLI flags. List-type settings accumulate across all layers; scalar settings use the most specific value.

## Environment variables

Set in shell config (`.zshrc`, `.bashrc`, etc.) for a persistent default. `.agenticrc.toml` and CLI flags override these - see [Precedence](#precedence) below.

| Variable                  | Description                                                                                                                                           | Default                                                  |
| ------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------- |
| `AGENTIC_HOME`            | Base directory for tool config and secrets                                                                                                            | `$HOME/.agentic`                                         |
| `AGENTIC_NAMESPACE`       | Image namespace. Images are named `<namespace>-<tool>`. Used when no `.agenticrc.toml` sets `namespace`.                                              | `agentic`                                                |
| `AGENTIC_EXTRA_MOUNTS`    | Comma-separated extra mounts. Bind mount: `host/path:container/path`. Named volume: `name:container/path` (auto-created). Supports `$CONTAINER_HOME`. | -                                                        |
| `AGENTIC_SECRETS`         | Comma-separated secrets to mount read-only into the container. Format: `name:/path/to/file[:/container/path]`. Defaults to `/run/secrets/<name>`.     | -                                                        |
| `AGENTIC_PIDS_LIMIT`      | Default container PID limit                                                                                                                           | `1024`                                                   |
| `AGENTIC_CPUS`            | Default container CPU limit                                                                                                                           | `4`                                                      |
| `AGENTIC_MEMORY`          | Default container memory limit                                                                                                                        | `4g`                                                     |
| `AGENTIC_<LAYER>_VERSION` | Version used when building the named runtime layer (e.g. `AGENTIC_JAVA_VERSION=17`, `AGENTIC_NODE_VERSION=22`)                                        | Embedded per-layer defaults (see `agentic build --help`) |

## `agentic.json` (global config)

Stored in `$AGENTIC_HOME/agentic.json` (default `~/.agentic/agentic.json`). Machine-level settings applied to all projects; edit directly with any text editor.

| Key                        | Type   | Description                                                                                                                   | CLI flag           |
| -------------------------- | ------ | ----------------------------------------------------------------------------------------------------------------------------- | ------------------ |
| `trusted_dirs`             | list   | Directories trusted to run tools from without an interactive prompt                                                           | `--trust-dir`      |
| `registry`                 | scalar | Registry prefix for base image pulls (e.g. `myregistry.example.com`). See below.                                              | `--registry`       |
| `docker_context`           | scalar | Machine-wide default Docker context. See [`docker_context`](#docker_context) below.                                           | `--docker-context` |
| `proxy_log_retention_days` | scalar | Days to keep egress proxy access logs before they're pruned automatically. Default: `3`.                                      | -                  |
| `audit_log_retention_days` | scalar | Days to keep filesystem audit logs before they're pruned automatically. Default: `3`.                                         | -                  |
| `last_update_check`        | scalar | Timestamp of the last automatic update check. Managed automatically - do not edit by hand.                                    | -                  |
| `last_tool_version_check`  | object | Per-tool timestamps of the last automatic tool-update check, keyed by tool name. Managed automatically - do not edit by hand. | -                  |

### Registry proxy

To pull Docker Hub images through a registry proxy (Harbor, Nexus, Artifactory, AWS ECR pull-through cache), set `registry`:

```json
{
  "registry": "myregistry.example.com"
}
```

Agentic prefixes all base image names with this value at build time. Authentication is out of scope - run `docker login myregistry.example.com` once.

`--registry` overrides `agentic.json` for a single build:

```bash
agentic build claude --registry myregistry.example.com
```

Run `agentic config` to see the active registry setting.

## `.agenticrc.toml` files

Place `.agenticrc.toml` in any directory to apply settings there and in subdirectories. `agentic` walks up from `$PWD`, collecting every `.agenticrc.toml` found, and stops at a file with `root = true` or the filesystem root.

### File format

Standard [TOML](https://toml.io). Build-time and runtime settings live in separate `[build]` and `[run]` sections; `root` and `namespace` are top-level keys.

```toml
# .agenticrc.toml
root = true

[build]
bases = ["java"]
apt_packages = ["make", "gcc", "jq"]

[build.versions]
java = "17"
node = "22"

[run]
extra_mounts = ["maven:$CONTAINER_HOME/.m2"]
env = ["NODE_OPTIONS=--max-old-space-size=4096"]
pids_limit = "2048"
```

### Keys

**Top-level**

| Key              | Type   | Description                                                                                                            | Env var             | Default   |
| ---------------- | ------ | ---------------------------------------------------------------------------------------------------------------------- | ------------------- | --------- |
| `root`           | bool   | Stop the upward directory walk at this file                                                                            | -                   | -         |
| `namespace`      | string | Image namespace. Images are named `<namespace>-<tool>` (e.g. `myproject-claude`). Allows multiple image sets per tool. | `AGENTIC_NAMESPACE` | `agentic` |
| `docker_context` | string | Docker context to use for this project. See [`docker_context`](#docker_context) below.                                 | -                   | -         |

**`[build]` section** - applied at `agentic build` / `agentic update` time

| Key               | Type           | Description                                                                                                                      | CLI flag    | Env var                   | Default |
| ----------------- | -------------- | -------------------------------------------------------------------------------------------------------------------------------- | ----------- | ------------------------- | ------- |
| `bases`           | list           | Extra runtime layers to add on top of the node base (e.g. `["java", "dotnet"]`). Accumulates across RC layers and with `--base`. | `--base`    | -                         | -       |
| `apt_packages`    | list           | Extra Debian packages to install in the base image. Accumulates across RC layers and with `--apt`.                               | `--apt`     | `AGENTIC_APT_PACKAGES`    | -       |
| `versions`        | TOML table     | Per-layer version pins. Written as `[build.versions]` with `node`, `java`, `dotnet`, or `go` keys. Innermost value wins per key. | `--<layer>` | `AGENTIC_<LAYER>_VERSION` | -       |
| `custom_installs` | list of tables | Non-apt tools installed via arbitrary shell commands. See `[[build.custom_installs]]` below.                                     | -           | -                         | -       |

**`[[build.custom_installs]]`** - non-apt tools (e.g. `helm`, `golangci-lint`) installed via arbitrary shell commands, applied unconditionally at build time - not gated by a `--<name>` flag the way `bases` extras are

| Key    | Type   | Description                                                                                                                                                                          | Default |
| ------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------- |
| `name` | string | Identifier for this install; must match `^[a-zA-Z0-9._-]+$`, unique across all merged `.agenticrc.toml` layers. Shown as a comment in the generated `RUN` and in `agentic config`.   | -       |
| `run`  | list   | Shell commands run as-is, in declaration order, in one Docker `RUN` layer per entry. No sandboxing, checksum, or allowlist - same trust level as a Dockerfile checked into the repo. | -       |

```toml
[[build.custom_installs]]
name = "helm"
run = [
  "curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 -o /tmp/get-helm.sh",
  "bash /tmp/get-helm.sh",
]
```

Each entry becomes its own Dockerfile stage `RUN`, inserted after any `--base` extras (`dotnet`/`go`/`java`/`node`) and before the tool's own install step - so a custom install can rely on any requested extra's toolchain being on `PATH` (e.g. a `go install`-based install can assume `--base go` already ran). rc-only: no CLI flag or env var equivalent, unlike `bases`/`apt_packages`. Unlike those two, `agentic update` does not ignore `custom_installs` from rc - it always reflects the current file, since there's no per-image label recovery for it (the `agentic.custom-installs` label is informational only, shown by `agentic inspect <tool>` - it's never read back to decide what to rebuild). Editing an entry's `run` always takes effect on the next `build`/`update` via normal Docker layer-cache invalidation on the changed `RUN` command.

`custom_installs` commands run as root, in a build stage before the container's non-root tool user is created (the same ordering `apt_packages`/`bases` already use). Install into a root-owned system path such as `/usr/local/bin` or `/opt` rather than `$HOME` - files written under the tool user's home directory will end up root-owned, and containers run `--read-only` as a non-root user at runtime, so such files would be unusable.

**`[run]` section** - applied at `agentic run` time

| Key                | Type   | Description                                                                                                                                                                                                                                                                                                                                                               | CLI flag            | Env var                | Default |
| ------------------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------- | ---------------------- | ------- |
| `extra_mounts`     | list   | Extra mounts passed to `docker run`. Bind: `host/path:container/path`. Named volume: `name:container/path`. Supports `~`, `$HOME`, `$TOOL_HOME`, `$CONTAINER_HOME`, `$PWD`                                                                                                                                                                                                | `-v`                | `AGENTIC_EXTRA_MOUNTS` | -       |
| `read_only_mounts` | list   | Sub-paths to force read-only even when their parent mount is writable. Format: `host/path:container/path` (`:ro` is always applied - any suffix you give is ignored), or a bare `sub/path` with no `:` as shorthand for a path relative to the workspace (expands to `$PWD/sub/path:/workspace/sub/path`). Supports `~`, `$HOME`, `$TOOL_HOME`, `$CONTAINER_HOME`, `$PWD` | `--read-only-mount` | -                      | -       |
| `secrets`          | list   | Files to mount read-only into the container. Format: `name:/path/to/file[:/container/path]`. Defaults to `/run/secrets/<name>`. Supports `~`, `$HOME`, `$CONTAINER_HOME` (container path only)                                                                                                                                                                            | `-s`                | `AGENTIC_SECRETS`      | -       |
| `env`              | list   | Environment variables to set in the container. Format: `KEY=VALUE`, or bare `KEY` to forward the host's current value. Cannot target a reserved name (see [env](#env) below)                                                                                                                                                                                              | `-e`                | -                      | -       |
| `pids_limit`       | string | Container PID limit (e.g. `"1024"`)                                                                                                                                                                                                                                                                                                                                       | `--pids-limit`      | `AGENTIC_PIDS_LIMIT`   | `1024`  |
| `cpus`             | string | Container CPU limit (e.g. `"4"`)                                                                                                                                                                                                                                                                                                                                          | `--cpus`            | `AGENTIC_CPUS`         | `4`     |
| `memory`           | string | Container memory limit (e.g. `"8g"`)                                                                                                                                                                                                                                                                                                                                      | `--memory`          | `AGENTIC_MEMORY`       | `4g`    |
| `check_updates`    | bool   | Periodically check upstream for a newer tool version during `agentic run` (at most once every 6 hours per tool) and offer to update. A pointer internally so an inner config can explicitly disable a check enabled by an outer one.                                                                                                                                      | -                   | -                      | `true`  |

**`[run.instructions]` section** - environment instructions written into each tool's global instructions file (see [Environment instructions](../README.md#-environment-instructions))

| Key       | Type   | Description                                                                                                                                                 | Default |
| --------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------- | ------- |
| `enabled` | bool   | Write the generated environment-instructions block. A pointer internally so an inner config can explicitly disable a block enabled by an outer one.         | `true`  |
| `custom`  | string | Free text appended after the generated sections. Accumulates across RC layers like a list key (outermost first, separated by a blank line), not overridden. | -       |

```toml
[run.instructions]
custom = """
Always run `go test ./...` before considering a task finished.
"""
```

Written into the tool's own global instructions file - `~/.claude/CLAUDE.md` (Claude Code), `~/.config/opencode/AGENTS.md` (OpenCode), `~/.copilot/copilot-instructions.md` (Copilot CLI) - a location each tool already reads automatically on startup, separate from any project-level `CLAUDE.md`/`AGENTS.md` you own in the repo, so it never collides with your own instructions.

The block is delimited by markers and only the content between them is ever replaced - anything else in the file, whether added by hand or by the tool itself at runtime (e.g. Claude Code's own memory/"remember this" feature), is left untouched across runs, including when `enabled = false` turns the block off entirely. Each run gets its own private, freshly-generated snapshot of the file for the container's lifetime, so concurrent runs of the same tool across projects never bleed instructions, resource limits, or proxy settings into each other; anything added to the file during a run is folded back into the persisted copy once the container exits.

Preview the exact content a run would write, without starting a container:

```bash
agentic instructions claude
agentic instructions claude --proxy
```

The generated block opens with a precedence note - it only describes the container environment, not coding conventions, so the project's own instructions file (`CLAUDE.md`, `AGENTS.md`, `copilot-instructions.md`) wins on conflicts. It then covers:

- **What's installed** - base toolchain (a static default, listed even before the image is built), extra runtimes, apt packages, and custom installs, read from the built image's labels so they reflect what's actually running, not a possibly stale `.agenticrc.toml`
- **What's restricted** - read-only filesystem, writable paths, resource limits, dropped privileges
- **Network access**, when the egress proxy is enabled - no direct internet access, plus the allowlist when enforced, with the same "tell the user so they can add it" note for a blocked host

**`[run.proxy]` section** - egress allowlist proxy

| Key             | Type   | Description                                                                                                                                                                                                               | CLI flag                 | Default     |
| --------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------ | ----------- |
| `enabled`       | bool   | Route the tool's egress through the allowlist proxy. `enabled` is a pointer internally so an inner config can explicitly disable a proxy enabled by an outer one.                                                         | `--proxy` / `--no-proxy` | `false`     |
| `mode`          | string | `"enforce"` blocks disallowed hosts; `"monitor"` logs the allowlist verdict without blocking anything. Setting `mode = "monitor"` implies the proxy is enabled, unless `enabled = false` is also set (which always wins). | `--proxy-monitor`        | `"enforce"` |
| `allowed_hosts` | list   | Extra hosts to permit, merged on top of the tool's baseline. Exact match (e.g. `"api.github.com"`), or a leading-dot / `*.` entry to match a domain and all its subdomains (e.g. `".github.com"`).                        | -                        | -           |

When enabled, the tool container loses direct internet access and reaches the outside only through a proxy sidecar on a per-run internal Docker network. Blocked hosts print at the end of the run; every attempt is logged as JSON lines under `$AGENTIC_HOME/proxy/`. The sidecar is reachable via auto-injected `HTTP_PROXY`/`HTTPS_PROXY`, or at the stable alias `agentic-proxy:3128` for tools needing a literal hostname (see [below](#pointing-a-tools-own-proxy-setting-at-the-egress-proxy)).

`--proxy-monitor` (or `mode = "monitor"`) never blocks - every host succeeds, including ones missing from the allowlist. The access log still records the real verdict (`"decision": "allow"` or `"deny"`), tagged `"enforced": false`; `docker logs` lines get a `(monitor)` suffix. At the end of the run, agentic reports the hosts that _would_ have been blocked, so you can fill the allowlist gap before switching to real `--proxy` enforcement. Use this to discover a new tool's egress needs before writing an allowlist.

Each proxy-enabled run prunes access logs older than a retention window (default 3 days), set via `proxy_log_retention_days` in `agentic.json` (host-level, not per-project). To wipe all logs regardless of age, run `agentic proxy clean --logs`.

Each tool ships a baseline allowlist that `allowed_hosts` merges on top of. The proxy image builds on demand on the first `--proxy` run, or explicitly via `agentic proxy build`/`agentic proxy update` (see [Development](05-development.md#building-the-proxy-image-locally)).

| Tool       | Baseline host        | Purpose                             |
| ---------- | -------------------- | ----------------------------------- |
| `claude`   | `.anthropic.com`     | Claude API and telemetry subdomains |
| `claude`   | `.claude.ai`         | installer and asset downloads       |
| `claude`   | `.claude.com`        | OAuth/login flow                    |
| `copilot`  | `.githubcopilot.com` | Copilot API and subdomains          |
| `copilot`  | `api.github.com`     | GitHub API used for authentication  |
| `opencode` | `opencode.ai`        | OpenCode auth and update checks     |

OpenCode is multi-provider, so only its own auth/update host is included by default - add your chosen model-provider hosts via `allowed_hosts`.

`agentic config` shows resolved `proxy.enabled`, `proxy.mode`, and `proxy.allowed_hosts` for the current directory, tagged with the `.agenticrc.toml` that set them (tool baseline hosts aren't included - they're fixed per tool, not configurable).

```toml
[run.proxy]
enabled = true
mode = "monitor" # or "enforce" (default)
allowed_hosts = [
  "registry.npmjs.org",
  ".github.com",
]
```

**`[run.audit]` section** - filesystem audit logging

| Key       | Type | Description                                                                                                                                                                                            | CLI flag                 | Default |
| --------- | ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------ | ------- |
| `enabled` | bool | Log filesystem activity under every bind-mounted host path for the run. `enabled` is a pointer internally so an inner config can explicitly disable auditing enabled by an outer one.                  | `--audit` / `--no-audit` | `false` |
| `exclude` | list | Extra directory names to skip while watching, merged with the built-in defaults (`.git`, `node_modules`, `vendor`, `dist`, `build`, `.venv`, `__pycache__`), to keep watch counts sane on large repos. | -                        | -       |

When enabled, agentic watches the host side of every bind mount directly for the container's lifetime - since a bind mount shares the host's underlying files, this sees all activity regardless of whether it came from the host or the container, with no extra privilege needed and no change to the container's own hardening. Activity (writes, creates, deletes, renames, and - Linux only, see below - opens) is logged as JSON lines under `$AGENTIC_HOME/audit/`; a one-line summary prints after the container exits, including a warning count if anything went wrong while watching (e.g. a root that couldn't be watched) even when no activity was recorded. Each audit-enabled run prunes logs older than a retention window (default 3 days), set via `audit_log_retention_days` in `agentic.json` (host-level, not per-project). The bare `agentic clean` (no tool argument) wipes all audit logs unconditionally as part of its global resource sweep.

**Platform support**: the watch mechanism is host-OS-specific, not container-specific (every container is still Linux regardless of host):

| Host OS | Backend   | Notes                                                                                                                                                                                                                                                        |
| ------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Linux   | `inotify` | Full fidelity - open, write, create, delete, and rename events, with the changed path given directly by the kernel.                                                                                                                                          |
| macOS   | `kqueue`  | Lower fidelity - no open events (macOS has no unprivileged equivalent), and a changed directory has to be re-listed and diffed against a cached listing to recover which entry changed, since kqueue only reports "this directory changed," not which entry. |
| Windows | none yet  | `--audit` / `enabled = true` returns a clear error rather than silently doing nothing.                                                                                                                                                                       |

`agentic config` shows resolved `audit.enabled` and `audit.exclude` for the current directory, tagged with the `.agenticrc.toml` that set them.

```toml
[run.audit]
enabled = true
exclude = ["target"] # merged with the built-in defaults
```

#### Pointing a tool's own proxy setting at the egress proxy

`HTTP_PROXY`/`HTTPS_PROXY` (and lowercase variants) are auto-injected whenever the proxy is enabled, so most tools need no extra configuration. Some tools ignore these env vars and require a literal host:port instead - Maven is an example: it only reads proxy settings from `settings.xml`'s `<proxies>` section, not `MAVEN_OPTS` or the standard proxy env vars.

For these cases, the sidecar is reachable at the stable hostname `agentic-proxy:3128` - unlike its actual Docker container name (randomized per run), this hostname is safe to hardcode once in the tool's own config. See [Tool-specific proxy examples](#tool-specific-proxy-examples) below for a Maven walkthrough.

**`[[marketplaces]]`** - git-based plugin marketplaces (skills, agents, commands, hooks, MCP servers, synced and mounted together as a single unit) to sync onto the host and mount read-only into every applicable tool's container

| Key     | Type   | Description                                                                                                                                                                                                                                                                                                     | Default               |
| ------- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------- |
| `name`  | string | Identifier for this marketplace. Must match `^[a-zA-Z0-9._-]+$` - becomes a container mount path segment. Not required to be globally unique: the host clone is keyed by `url` alone, so two projects may give the same `url` different `name`s (or the same `name` to different `url`s) without any collision. | -                     |
| `url`   | string | Git URL to clone (anything `git clone` accepts - `https://`, `git@host:...`, etc).                                                                                                                                                                                                                              | -                     |
| `tools` | list   | Tool names this marketplace is mounted into (currently `claude`, `copilot`). Omit to mount into every tool that supports marketplaces.                                                                                                                                                                          | every supporting tool |

```toml
[[marketplaces]]
name = "acme-plugins"
url  = "git@github.com:acme/plugin-marketplace.git"
# tools omitted -> mounted into every tool that supports it (claude, copilot)

[[marketplaces]]
name = "claude-only-thing"
url  = "git@github.com:acme/claude-extras.git"
tools = ["claude"]
```

Before each `agentic run`/`agentic <tool>`, every applicable marketplace is synced on the host - cloned if new, or `git fetch` + `git reset --hard @{upstream}` if already cloned - using the invoking user's own git auth (SSH agent, credential helper, `~/.netrc`); no credentials enter the container. Requires `git` on the host `PATH`. A failed first clone fails the run; a failed fetch on an already-cloned marketplace just warns and reuses the (possibly stale) existing clone.

The clone is bind-mounted read-only, so the container itself never touches git. Clones are shared across tools and projects: the host path is `$AGENTIC_HOME/marketplaces/<slug>-<hash>/`, a pure function of `url` (`<hash>` a short hash of it, `<slug>` a label derived from it - not from `name`), so any project referencing the same `url` reuses the same clone regardless of the local `name` it gives it. Each tool mounts it at the neutral, agentic-owned path `~/marketplaces/<name>` rather than inside its own config tree, so it never collides with marketplace state a tool persists for itself.

Mounting isn't registering: each tool only recognizes marketplaces it has explicitly added to its own config, not whatever is present on disk. So each tool's `entrypoint.sh` adds every currently-mounted name (`claude`/`copilot plugin marketplace add <dir>`) and removes any previously-registered, agentic-managed marketplace that's no longer mounted (`... remove <name>`) - so deleting a `[[marketplaces]]` entry cleans itself up the next time that project runs. Copilot warns rather than fails on error, and (unlike Claude) has no `--json` list output, so its entrypoint parses the plain `NAME (Local: PATH)` lines from `copilot plugin marketplace list` instead - see `internal/tools/copilot.go`.

Usage is tracked in `$AGENTIC_HOME/marketplaces/.usage.json`, keyed by clone + local `name`, so multiple projects can share a clone under different names. `agentic marketplaces list` shows every clone and which project(s)/name(s) reference it (a clone under two names shows as two rows sharing a URL); `agentic marketplaces prune` deletes clones with no live project reference left, re-checking each recorded project's current config first - a clone survives as long as any one of its recorded names is still live. A clone with no usage record (e.g. placed manually) is left alone. `agentic clean` does not touch marketplace clones.

### Merge semantics

Multiple `.agenticrc.toml` files merge. The walk starts at `$PWD` and moves upward, so the file closest to the root is the _outermost_ and the file in `$PWD` is the _innermost_.

- **List keys** (`bases`, `apt_packages`, `custom_installs`, `extra_mounts`, `read_only_mounts`, `secrets`, `env`, `proxy.allowed_hosts`, `audit.exclude`, `marketplaces`): values from all levels accumulate, outermost first.
- **Scalar keys** (`pids_limit`, `cpus`, `memory`, `namespace`, `docker_context`): the innermost (child) value wins; outer files fill in any keys the inner file does not set.
- **`instructions.custom`**: text from all levels accumulates like a list key (outermost first, joined by a blank line), rather than the innermost overriding it - each layer's text is additive context, not a single setting. `instructions.enabled` is a scalar key: the innermost (child) value wins.
- **`versions` table**: each layer name is resolved independently - innermost value wins per key, so a child can pin `java` without affecting `node` inherited from a parent.

```
~/projects/.agenticrc.toml              ← outermost (root=true stops the walk here)
~/projects/my-project/.agenticrc.toml  ← innermost ($PWD)
```

Given these two files:

```toml
# ~/projects/.agenticrc.toml
root = true

[build]
apt_packages = ["make"]

[run]
cpus = "4"
```

```toml
# ~/projects/my-project/.agenticrc.toml
[build]
apt_packages = ["gcc"]

[run]
cpus = "8"
```

The effective configuration is `apt_packages = ["make", "gcc"]` and `cpus = "8"` (child wins for scalars).

## Precedence

### `apt_packages`

Packages accumulate across all three sources in this order:

1. `.agenticrc.toml` files (outermost first)
2. `AGENTIC_APT_PACKAGES` environment variable (comma-separated)
3. `--apt` flag

Duplicates are removed while preserving order. The resolved list is verified with `apt-cache show` before the build starts (fail-fast).

### `bases`

Extra runtime layers accumulate across RC files and the `--base` flag:

1. `.agenticrc.toml` files (outermost first)
2. `--base` flag (appended, deduplicated)

`AGENTIC_BASE_OVERRIDE` is a full override - when set it replaces all RC and flag values.

### `versions`

Per-layer version resolution (highest to lowest priority):

1. `--<layer>` flag (e.g. `--java 17`) or `AGENTIC_<LAYER>_VERSION` env var
2. `.agenticrc.toml` `[build.versions]` - innermost value wins per key
3. Built-in default (from the bundled `versions.json`)

### `extra_mounts` and `secrets`

These accumulate too; their env vars (`AGENTIC_EXTRA_MOUNTS`, `AGENTIC_SECRETS`) and RC values are collected independently and combined at runtime.

### `read_only_mounts`

Each entry forces one sub-path read-only, even though its parent directory (`$PWD`, a tool's own state dir, ...) stays writable - useful for keeping a credentials sub-directory or similar off-limits to writes without splitting it into a separate, fully-read-only mount elsewhere. Under the hood this relies on plain Docker bind-mount behavior: agentic places `read_only_mounts` entries last in the assembled mount list, so they shadow any overlapping read-write mount for that sub-path specifically (the same mechanism marketplace mounts already use to stay read-only alongside a tool's writable state). Order in the TOML file itself doesn't matter - agentic always places these last regardless of where they appear.

A bare entry with no `:` is shorthand for a path relative to the workspace - it expands to the same sub-path on both sides, under `$PWD` on the host and `/workspace` in the container:

```toml
[run]
read_only_mounts = [".git", ".credentials"]
```

is equivalent to:

```toml
[run]
read_only_mounts = ["$PWD/.git:/workspace/.git", "$PWD/.credentials:/workspace/.credentials"]
```

`--read-only-mount` sets the same thing per invocation and accumulates with the config list (`--read-only-mount` values first, then `read_only_mounts` - no env var for this one, unlike `extra_mounts`/`secrets`).

### `env`

`.agenticrc.toml` `env` entries and `-e`/`--env` flags accumulate, but on a duplicate key `-e` wins - RC entries apply first, and the last `--env` for a given key takes effect, matching `docker run -e` itself.

`-e`/`--env` values are visible inside the container and via `docker inspect`/`ps` - use `-s`/`--secret` for tokens or credentials instead.

`TZ` is auto-forwarded from the host's detected timezone, alongside the terminal-capability vars (`COLORTERM`, `TERM`, `NO_COLOR`, `FORCE_COLOR`); an explicit `-e TZ=...` or `.agenticrc.toml` `env` entry overrides it like any other auto-forwarded var.

### `namespace`

Resolution priority (highest to lowest):

1. `--namespace` flag
2. `.agenticrc.toml` `namespace` - innermost (child) value wins
3. `AGENTIC_NAMESPACE` environment variable
4. Built-in default (`agentic`)

With the default namespace, images are named `agentic-claude`, `agentic-copilot`, etc.

Example: building separate images for a Java project:

```toml
# ~/projects/java-app/.agenticrc.toml
namespace = "java-app"

[build]
bases = ["java"]
apt_packages = ["make"]

[build.versions]
java = "17"
```

Then `agentic build claude` creates `java-app-claude` with the Java layer, while the default `agentic-claude` remains untouched.

### `docker_context`

Selects which [Docker context](https://docs.docker.com/engine/manage-resources/contexts/) `agentic` talks to - useful with multiple Docker contexts (e.g. local + remote) when you want a specific one instead of whatever's currently active.

Resolution priority (highest to lowest):

1. `--docker-context` flag
2. `.agenticrc.toml` `docker_context` - innermost (child) value wins
3. `agentic.json` `docker_context` (machine-wide default)
4. Unset - defers to the docker CLI's own context resolution, unchanged (including its `DOCKER_CONTEXT` environment variable, if set)

```toml
# .agenticrc.toml
docker_context = "prod"
```

`--docker-context` tab-completes against `docker context ls`. `agentic status` prints the active context (when non-default) above the container table, and `agentic config` shows the resolved value from both `agentic.json` and the merged `.agenticrc.toml` layers.

### Scalar settings (`pids_limit`, `cpus`, `memory`)

Resolution priority (highest to lowest):

1. CLI flag (`--pids-limit`, `--cpus`, `--memory`) on `agentic run`
2. `.agenticrc.toml` - innermost (child) value wins
3. Environment variable (`AGENTIC_PIDS_LIMIT`, `AGENTIC_CPUS`, `AGENTIC_MEMORY`)
4. Built-in default (`1024`, `4`, `4g`)

## Using `root = true`

`root = true` marks a boundary in the directory walk - useful for monorepos with a shared config at the repo root and per-project configs in subdirectories, without picking up configs from outside the repo:

```toml
# ~/projects/.agenticrc.toml - shared config for all projects
root = true

[build]
apt_packages = ["make"]

[run]
secrets = ["gh-token:~/.secrets/gh_token"]
```

```toml
# ~/projects/my-project/.agenticrc.toml - project-specific additions
[build]
apt_packages = ["gcc"]

[run]
extra_mounts = ["maven:$CONTAINER_HOME/.m2"]
cpus = "8"
```

Running `agentic` from `~/projects/my-project` merges both files and stops; `~/projects` is not traversed further even if a `.agenticrc.toml` exists above it.

## Mount variable expansion

These placeholders expand in mount strings (`extra_mounts`, `read_only_mounts`, `AGENTIC_EXTRA_MOUNTS`, `-v`, `--read-only-mount`) at runtime, so paths aren't hardcoded per machine or per tool:

| Placeholder         | Side of `:`       | Expands to                                                                  |
| ------------------- | ----------------- | --------------------------------------------------------------------------- |
| `~`                 | host (left)       | Your home directory                                                         |
| `$HOME`             | host (left)       | Same as above                                                               |
| `${HOME}`           | host (left)       | Same as above                                                               |
| `$TOOL_HOME`        | host (left)       | Agentic data directory (e.g. `~/.agentic`)                                  |
| `${TOOL_HOME}`      | host (left)       | Same as above                                                               |
| `$PWD`              | host (left)       | Current working directory (the same directory bind-mounted as `/workspace`) |
| `${PWD}`            | host (left)       | Same as above                                                               |
| `$CONTAINER_HOME`   | container (right) | Container home directory (e.g. `/home/claude`)                              |
| `${CONTAINER_HOME}` | container (right) | Same as above                                                               |

Use single quotes (or escape the `$`) so the shell doesn't expand the variables before passing them to `agentic`:

```bash
agentic -v '$TOOL_HOME/custom:$CONTAINER_HOME/.custom' claude
export AGENTIC_EXTRA_MOUNTS='~/.m2:$CONTAINER_HOME/.m2,~/.gradle:$CONTAINER_HOME/.gradle'
```

## Inspecting the merged config

Run `agentic config` to see the merged result of all active `.agenticrc.toml` files and environment variables for the current directory:

```
agentic config
```

## Tool-specific proxy examples

Concrete walkthroughs for routing a tool's own proxy setting through the `agentic-proxy:3128` egress sidecar (see [Pointing a tool's own proxy setting at the egress proxy](#pointing-a-tools-own-proxy-setting-at-the-egress-proxy)).

### Maven

Maven only reads proxy settings from `settings.xml`'s `<proxies>` section, not `MAVEN_OPTS` or the standard proxy env vars. Mount a `settings.xml` pointing at `agentic-proxy:3128`, with a `<proxy>` entry per URL scheme - Maven matches `<protocol>` against the repository URL (not the connection to the proxy itself), and most registries including Maven Central serve over `https`:

```xml
<!-- settings.xml -->
<settings>
  <proxies>
    <proxy>
      <id>agentic-proxy-http</id>
      <active>true</active>
      <protocol>http</protocol>
      <host>agentic-proxy</host>
      <port>3128</port>
    </proxy>
    <proxy>
      <id>agentic-proxy-https</id>
      <active>true</active>
      <protocol>https</protocol>
      <host>agentic-proxy</host>
      <port>3128</port>
    </proxy>
  </proxies>
</settings>
```

```toml
# .agenticrc.toml
[run]
secrets = ["maven-settings:~/.m2/settings.xml:$CONTAINER_HOME/.m2/settings.xml"]

[run.proxy]
enabled = true
allowed_hosts = ["repo.maven.apache.org"]
```

Pointing `<proxies>` at an _external_ corporate proxy instead would bypass agentic's egress allowlist entirely, since that traffic never reaches the `agentic-proxy` sidecar - only routing through `agentic-proxy` keeps Maven's traffic subject to `allowed_hosts`.
