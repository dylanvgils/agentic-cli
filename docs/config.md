# Configuration

Agentic reads settings from three places:

- `agentic.json` holds machine-wide settings. Trust and approvals live next to it in [`state.json`](#statejson).
- `.agenticrc.toml` files hold per-directory settings.
- CLI flags hold per-run settings.

For a scalar setting, the most specific value wins. List settings add up across all three (see [Precedence](#precedence)). Run `agentic config` to see the merged result for the current directory and which file set each value.

`AGENTIC_HOME` (default `$HOME/.agentic`) is a plain environment variable. It has to be known before any config file can be found.

## `agentic.json`

Stored at `$AGENTIC_HOME/agentic.json`. Edit it with any text editor; agentic only reads it.

| Key                        | Type   | Description                                                             | CLI flag           |
| -------------------------- | ------ | ----------------------------------------------------------------------- | ------------------ |
| `registry`                 | scalar | Registry prefix for base image pulls. See [Registry proxy](#registry-proxy). | `--registry`       |
| `docker_context`           | scalar | Machine-wide default Docker context. See [Precedence](#precedence).     | `--docker-context` |
| `proxy_log_retention_days` | scalar | Days to keep [egress proxy](egress-proxy.md) logs. Default: `3`.        | -                  |

### Registry proxy

To pull Docker Hub images through a registry proxy (Harbor, Nexus, Artifactory, an ECR pull-through cache), set `registry`. Agentic prefixes every base image name with it. Run `docker login` for the registry yourself.

```json
{
  "registry": "myregistry.example.com"
}
```

`--registry` overrides it for one build: `agentic build claude --registry myregistry.example.com`.

## `state.json`

Agentic records what it learns at `$AGENTIC_HOME/state.json`, apart from your settings. Delete an entry to revoke it, or the whole file to reset all trust and approvals. Older versions kept these keys in `agentic.json`; the next `agentic` command moves them here.

| Key                       | Description                                                               |
| ------------------------- | ------------------------------------------------------------------------- |
| `trusted_dirs`            | Directories you trusted, by prompt or `--trust-dir`, stored as real paths |
| `approved_settings`       | Your [setting approvals](#setting-approval) per `.agenticrc.toml`         |
| `last_update_check`       | When agentic last checked for a new release                               |
| `last_tool_version_check` | When agentic last checked each tool for an update                         |

## `.agenticrc.toml`

A `.agenticrc.toml` applies to its directory and all subdirectories. `agentic` walks up from `$PWD` and collects every file it finds. The walk stops at a file with `root = true`, or at the filesystem root. See [Merge semantics](#merge-semantics) for how the files combine.

```toml
# .agenticrc.toml
root = true

[build]
bases = ["java"]
apt_packages = ["make", "gcc", "ripgrep"]

[build.versions]
java = "17"
node = "22"

[run]
mounts = ["maven:$CONTAINER_HOME/.m2"]
env = ["NODE_OPTIONS=--max-old-space-size=4096"]
pids_limit = "2048"
```

### Editor validation

Add a `#:schema` comment on the first line to get validation and autocomplete from [`taplo`](https://taplo.tamasfe.dev/)-based editors, such as VS Code's "Even Better TOML" extension or Neovim's `taplo` server:

```toml
#:schema https://raw.githubusercontent.com/dylanvgils/agentic-cli/main/agenticrc.schema.json
```

Inside this repo, use the local file instead: `#:schema ./agenticrc.schema.json`.

### Top-level keys

| Key              | Type   | Description                                                                                  | Default   |
| ---------------- | ------ | -------------------------------------------------------------------------------------------- | --------- |
| `root`           | bool   | Stop the upward directory walk at this file                                                  | -         |
| `namespace`      | string | Image namespace. Images are named `<namespace>-<tool>`. See [Per-project image set](recipes.md#per-project-image-set). | `agentic` |
| `docker_context` | string | [Docker context](usage.md#docker-context) to use for this project                            | -         |

### `[build]`

Applied by `agentic build`. See [Images](images.md) for what each setting does to the image, and what `agentic update` keeps.

| Key               | Type           | Description                                                                 | CLI flag                  |
| ----------------- | -------------- | --------------------------------------------------------------------------- | ------------------------- |
| `bases`           | list           | Extra runtime layers (e.g. `["java", "dotnet"]`)                            | `--base` / `--base-exact` |
| `apt_packages`    | list           | Extra Debian packages                                                       | `--apt` / `--apt-exact`   |
| `versions`        | table          | `[build.versions]` pins for `node`, `java`, `dotnet`, `go` or `docker`      | `--<layer>`               |
| `custom_installs` | list of tables | Tools installed with shell commands. See below.                             | -                         |

### `[[build.custom_installs]]`

Installs a tool that isn't available through apt (e.g. `helm`, `golangci-lint`).

| Key    | Type   | Description                                                                                        |
| ------ | ------ | -------------------------------------------------------------------------------------------------- |
| `name` | string | Identifier, must match `^[a-zA-Z0-9._-]+$` and be unique across all merged files. Shown in the generated `RUN` and in `agentic config`. |
| `run`  | list   | Shell commands, each its own `RUN` layer, run as-is. No sandboxing, checksum or allowlist: the same trust level as a Dockerfile in the repo. |

```toml
[[build.custom_installs]]
name = "helm"
run = [
  "curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 -o /tmp/get-helm.sh",
  "bash /tmp/get-helm.sh",
]
```

- These run after the `bases` layers and before the tool install, so they can use any requested runtime (e.g. `go install` with `--base go`).
- Commands run as root. Install into `/usr/local/bin` or `/opt`, not `$HOME`, because the container user can't use root-owned files in its home.
- There's no CLI flag for this setting. Editing `run` rebuilds that layer on the next `agentic build`.

### `[run]`

Applied by `agentic run`.

| Key             | Type   | Description                                                                                                        | CLI flag       | Default    |
| --------------- | ------ | ------------------------------------------------------------------------------------------------------------------ | -------------- | ---------- |
| `mounts`        | list   | Bind mounts or named volumes, with the same syntax as [`--volume`](usage.md#named-volumes)                         | `--volume`     | -          |
| `secrets`       | list   | Files mounted read-only, with the same format as [`--secret`](usage.md#secrets)                                    | `--secret`     | -          |
| `env`           | list   | `KEY=VALUE`, or bare `KEY` to forward the host value. See [Environment variables](usage.md#environment-variables). | `--env`        | -          |
| `runtime`       | string | `"docker"`, or `"microsandbox"` to run in a [microVM](microsandbox.md)                                             | `--runtime`    | `"docker"` |
| `pids_limit`    | string | Container PID limit                                                                                                | `--pids-limit` | `1024`     |
| `cpus`          | string | Container CPU limit                                                                                                | `--cpus`       | `4`        |
| `memory`        | string | Container memory limit (e.g. `"8g"`)                                                                               | `--memory`     | `4g`       |
| `check_updates` | bool   | Check for a newer tool version on run. See [Versions and updates](images.md#versions-and-updates).                 | -              | `true`     |

Mount paths support [placeholders](#mount-variable-expansion). Limits in files must be positive, as docker reads zero or below as no limit; only the flags can lift them.

### `[run.instructions]`

Controls the environment-instructions block that each run writes into the tool's own global instructions file:

| Tool        | File                                |
| ----------- | ----------------------------------- |
| Claude Code | `~/.claude/CLAUDE.md`               |
| OpenCode    | `~/.config/opencode/AGENTS.md`      |
| Copilot CLI | `~/.copilot/copilot-instructions.md` |

| Key       | Type   | Description                                                   | Default |
| --------- | ------ | ------------------------------------------------------------- | ------- |
| `enabled` | bool   | Write the block                                               | `true`  |
| `custom`  | string | Text appended after the generated sections                    | -       |

```toml
[run.instructions]
custom = """
Always run `go test ./...` before considering a task finished.
"""
```

The block tells the model about its environment, and says the project's own `CLAUDE.md`/`AGENTS.md` wins on conflicts. It covers:

- **What's installed**: the base toolchain, runtimes, apt packages and custom installs, read from the built image. The base toolchain is listed even before the first build.
- **What's restricted**: the read-only filesystem, writable paths, resource limits and dropped privileges.
- **Network**, with `--proxy`: no direct internet access, the allowlist, and a note to tell you about blocked hosts so you can allow them.
- **Docker**, with `--dind`: how to reach the daemon and its ports, and which paths it can bind mount.

The rest of the file is left alone, including notes the tool saves there itself, even with `enabled = false`. Each run works on a private copy of the file, so concurrent runs don't affect each other. Changes made during the run are copied back when the container exits.

The file must be a regular file: agentic refuses a symlink there, since the tool can write that directory and could otherwise redirect the copy-back to any host file. Reads and writes also stay inside `$AGENTIC_HOME/tools`.

Preview the block with `agentic instructions claude` (add `--proxy` to include the network section).

### `[run.proxy]`

| Key             | Type   | Description                                                                                       | CLI flag                 | Default     |
| --------------- | ------ | ------------------------------------------------------------------------------------------------- | ------------------------ | ----------- |
| `enabled`       | bool   | Route egress through the allowlist proxy                                                          | `--proxy` / `--no-proxy` | `false`     |
| `mode`          | string | `"enforce"` blocks hosts not on the allowlist, `"monitor"` only logs them. `"monitor"` turns the proxy on unless `enabled = false`. | `--proxy-monitor`        | `"enforce"` |
| `allowed_hosts` | list   | Hosts added to the tool's baseline. Exact (`"api.github.com"`), or `.`/`*.` prefix for a domain and its subdomains (`".github.com"`). | -                        | -           |

### `[[run.proxy.credentials]]`

| Key      | Type   | Description                                                                                              |
| -------- | ------ | -------------------------------------------------------------------------------------------------------- |
| `preset` | string | `anthropic`, `openai` or `github`. Can't be combined with `hosts` or `header`.                           |
| `hosts`  | list   | Exact hostnames or IPs, no wildcards. Required without `preset`. A host may appear in only one entry.    |
| `header` | string | Header the tool sends the placeholder in, e.g. `"Authorization"`. Required without `preset`. Hop-by-hop, `Host` and `Content-Length` are refused. |
| `env`    | list   | Tool env vars set to the placeholder. Required without `preset`; replaces a preset's own list.          |
| `secret` | string | Required. Host file with the secret: an absolute path, or one starting with `~`/`$HOME`. One line, at most 64 KiB. Other sources (`keychain:`) are reserved. |

| Preset      | Hosts and header                                                                              | Env                        |
| ----------- | --------------------------------------------------------------------------------------------- | -------------------------- |
| `anthropic` | `api.anthropic.com`: `X-Api-Key`                                                              | `ANTHROPIC_API_KEY`        |
| `openai`    | `api.openai.com`: `Authorization`                                                             | `OPENAI_API_KEY`           |
| `github`    | `api.github.com`, `github.com`, `api.githubcopilot.com`, `api.{individual,business,enterprise}.githubcopilot.com`: `Authorization`; a git credential helper as `x-access-token` | `GITHUB_TOKEN`, `GH_TOKEN` |

```toml
[[run.proxy.credentials]]
preset = "anthropic"
secret = "~/.secrets/anthropic_key"

[[run.proxy.credentials]]
hosts = ["api.example.com"]
header = "Authorization"
env = ["EXAMPLE_TOKEN"]
secret = "~/.secrets/example_token"
```

### `[run.dind]`

| Key          | Type   | Description                                                             | CLI flag               | Default      |
| ------------ | ------ | ----------------------------------------------------------------------- | ---------------------- | ------------ |
| `enabled`    | bool   | Start a Docker daemon: a [sidecar](docker-in-docker.md) (needs `--base docker`) or [in the VM](microsandbox.md#docker-in-the-vm). | `--dind` / `--no-dind` | `false`      |
| `pids_limit` | string | Sidecar PID limit (Docker runtime only)                                 | `--dind-pids-limit`    | tool's value |
| `cpus`       | string | Sidecar CPU limit (Docker runtime only)                                 | `--dind-cpus`          | tool's value |
| `memory`     | string | Sidecar memory limit (e.g. `"8g"`, Docker runtime only)                 | `--dind-memory`        | tool's value |

### `[[marketplaces]]`

Git-based plugin marketplaces to sync on the host and mount read-only into tool containers. See [Marketplaces](usage.md#marketplaces) for how syncing and cleanup work.

| Key     | Type   | Description                                                                                     | Default               |
| ------- | ------ | ----------------------------------------------------------------------------------------------- | --------------------- |
| `name`  | string | Local name, must match `^[a-zA-Z0-9._-]+$`. Used as the mount path `~/marketplaces/<name>`.     | -                     |
| `url`   | string | `https://`, `ssh://` or `user@host:path`; local paths and other transports are refused          | -                     |
| `tools` | list   | Tools to mount it into (`claude`, `copilot`)                                                    | every supporting tool |

```toml
[[marketplaces]]
name = "acme-plugins"
url  = "git@github.com:acme/plugin-marketplace.git"

[[marketplaces]]
name = "claude-only-thing"
url  = "git@github.com:acme/claude-extras.git"
tools = ["claude"]
```

## Setting approval

The agent can edit `.agenticrc.toml` files in directories it ran in, so changes to these keys in any of them need your approval on the next run. The prompt shows what each changed key controls and its new value. Non-interactive runs fail until approved, and a config file symlinked to another project's asks again.

- `root`, `namespace`, `docker_context`, `marketplaces`
- `build.custom_installs`
- `run.mounts`, `run.secrets`, `run.env`, `run.runtime`
- `run.proxy.enabled`, `run.proxy.mode`, `run.proxy.allowed_hosts`, `run.proxy.credentials`, `run.dind.enabled`

`agentic build` and `agentic update` don't ask, so review the diff first.

## Merge semantics

The file closest to the filesystem root is the _outermost_. The file in `$PWD` is the _innermost_.

- **List keys** (`bases`, `apt_packages`, `custom_installs`, `mounts`, `secrets`, `env`, `proxy.allowed_hosts`, `proxy.credentials`, `marketplaces`) add up, outermost first.
- **Scalar keys** take the innermost value. Booleans count as scalars, so `enabled = false` in an inner file overrides an outer `true`.
- **`versions`** is resolved per key, so a child can pin `java` and still inherit `node`. An empty value counts as unset.
- **`instructions.custom`** adds up like a list, joined by blank lines.

Example, with `root = true` keeping configs above `~/projects` out:

```toml
# ~/projects/.agenticrc.toml (outermost)
root = true

[build]
apt_packages = ["make"]

[run]
cpus = "4"
```

```toml
# ~/projects/my-project/.agenticrc.toml (innermost)
[build]
apt_packages = ["gcc"]

[run]
cpus = "8"
```

Result: `apt_packages = ["make", "gcc"]` and `cpus = "8"`.

## Precedence

The first source that sets a scalar wins:

| Setting                        | Resolution order                                                                         |
| ------------------------------ | ---------------------------------------------------------------------------------------- |
| `namespace`                    | `--namespace`, then the innermost file, then `agentic`                                   |
| `versions`                     | `--<layer>` (e.g. `--java 17`), then `[build.versions]`, then the bundled default        |
| `runtime`                      | `--runtime`, then the innermost file, then `docker`                                      |
| `pids_limit`, `cpus`, `memory` | flag, then the innermost file, then `1024` / `4` / `4g`                                  |
| `[run.dind]` limits            | `--dind-*` flag, then `[run.dind]`, then the tool's resolved value                       |
| `docker_context`               | `--docker-context`, then the innermost file, then `agentic.json`, then the Docker CLI's own choice (including `DOCKER_CONTEXT`) |

List settings combine instead:

| Setting                   | How it combines                                                                     |
| ------------------------- | ----------------------------------------------------------------------------------- |
| `bases`, `apt_packages`   | Files (outermost first), then `--base`/`--apt`. Duplicates removed.                 |
| `mounts`, `secrets`       | Files and `--volume`/`--secret` combined                                            |
| `env`                     | Files, then `--env`. The last value for a key wins. Some names are [reserved](usage.md#environment-variables). |

`--base-exact` and `--apt-exact` replace the list instead of adding to it. They ignore config files and, during `agentic update`, the list the image was built with. `--base-exact=` or `--apt-exact=` (empty) means none at all. Each can't be combined with its non-exact flag.

## Mount variable expansion

Mount strings (`mounts`, `--volume`) expand these placeholders at runtime. The `${VAR}` form works too.

| Placeholder       | Side of `:`       | Expands to                                 |
| ----------------- | ----------------- | ------------------------------------------ |
| `~`, `$HOME`      | host (left)       | Your home directory                        |
| `$TOOL_HOME`      | host (left)       | Agentic data directory (e.g. `~/.agentic`) |
| `$PWD`            | host (left)       | Current directory, mounted as `/workspace` |
| `$CONTAINER_HOME` | container (right) | Container home (e.g. `/home/claude`)       |

Single-quote them so your shell doesn't expand them first:

```bash
agentic run --volume '$TOOL_HOME/custom:$CONTAINER_HOME/.custom:rw' claude
```

## Mount safety

Before a run starts, again right before the sidecars start, and once more right before the tool starts (after the sidecars are up or the microsandbox image is loaded), agentic checks every bind mount and secret:

- A mount that would expose `$AGENTIC_HOME/agentic.json` or `state.json` (the agentic home or a parent, such as `~`) is refused, since they hold machine-wide settings, trusted directories and approvals.
- A path through a symlink inside the workspace is refused, so a planted `.git -> ~/.ssh` can't be mounted via `mounts = ["$PWD/.git:/workspace/.git"]`. Mount the real path instead.
- A path under `/proc`, or one agentic can't resolve, is refused.

Not covered:

- A path swapped for a symlink in the moment between a check and Docker or msb mounting it, such as while the proxy starts before the DinD sidecar mounts the workspace. Only something already running in the workspace can do this, such as an agent from another run in the same project.
- A symlink planted inside another `:rw` mount.
