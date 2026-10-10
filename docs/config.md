# Configuration

Agentic reads settings from three places:

- `agentic.json` holds machine-wide settings.
- `.agenticrc.toml` files hold per-directory settings.
- CLI flags hold per-run settings.

For a scalar setting, the most specific value wins. List settings add up across all three (see [Precedence](#precedence)). Run `agentic config` to see the merged result for the current directory and which file set each value.

`AGENTIC_HOME` (default `$HOME/.agentic`) is a plain environment variable. It has to be known before any config file can be found.

## `agentic.json`

Stored at `$AGENTIC_HOME/agentic.json`. Edit it with any text editor.

| Key                        | Type   | Description                                                             | CLI flag           |
| -------------------------- | ------ | ----------------------------------------------------------------------- | ------------------ |
| `trusted_dirs`             | list   | Directories you can run tools from without an interactive trust prompt, saved by real path so a retargeted symlink asks again | `--trust-dir`      |
| `registry`                 | scalar | Registry prefix for base image pulls. See [Registry proxy](#registry-proxy). | `--registry`       |
| `docker_context`           | scalar | Machine-wide default Docker context. See [Precedence](#precedence).     | `--docker-context` |
| `proxy_log_retention_days` | scalar | Days to keep [egress proxy](egress-proxy.md) logs. Default: `3`.        | -                  |
| `last_update_check`        | scalar | Managed automatically.                                                  | -                  |
| `last_tool_version_check`  | object | Managed automatically.                                                  | -                  |
| `approved_credentials`     | object | Your [credential](egress-proxy.md#credential-injection) approvals. Managed automatically. | -                  |

### Registry proxy

To pull Docker Hub images through a registry proxy (Harbor, Nexus, Artifactory, an ECR pull-through cache), set `registry`. Agentic prefixes every base image name with it. Run `docker login` for the registry yourself.

```json
{
  "registry": "myregistry.example.com"
}
```

`--registry` overrides it for one build: `agentic build claude --registry myregistry.example.com`.

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
extra_mounts = ["maven:$CONTAINER_HOME/.m2"]
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

| Key                | Type   | Description                                                                                                       | CLI flag            | Default |
| ------------------ | ------ | ----------------------------------------------------------------------------------------------------------------- | ------------------- | ------- |
| `extra_mounts`     | list   | Bind mounts or named volumes, with the same syntax as [`-v`](usage.md#named-docker-volumes)                       | `-v`                | -       |
| `read_only_mounts` | list   | Sub-paths forced read-only inside a writable mount. See below.                                                    | `--read-only-mount` | -       |
| `secrets`          | list   | Files mounted read-only, with the same format as [`-s`](usage.md#secrets)                                         | `-s`                | -       |
| `env`              | list   | `KEY=VALUE`, or bare `KEY` to forward the host value. See [Environment variables](usage.md#environment-variables). | `-e`                | -       |
| `pids_limit`       | string | Container PID limit                                                                                               | `--pids-limit`      | `1024`  |
| `cpus`             | string | Container CPU limit                                                                                               | `--cpus`            | `4`     |
| `memory`           | string | Container memory limit (e.g. `"8g"`)                                                                              | `--memory`          | `4g`    |
| `check_updates`    | bool   | Check for a newer tool version on run. See [Versions and updates](images.md#versions-and-updates).               | -                   | `true`  |

Mount paths support [placeholders](#mount-variable-expansion).

#### `read_only_mounts`

Each entry makes one sub-path read-only while its parent mount stays writable, e.g. to protect a credentials directory. Entries always mount last, so they win over any overlapping mount. `:ro` is always applied, so any suffix you add is ignored. A bare entry with no `:` is relative to the workspace:

```toml
[run]
read_only_mounts = [".git", ".credentials"]
# same as ["$PWD/.git:/workspace/.git", "$PWD/.credentials:/workspace/.credentials"]
```

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
- **Docker**, with `--dind`: how to reach the sidecar daemon and its ports, and that only `/workspace` is shared with it.

The rest of the file is left alone, including notes the tool saves there itself, even with `enabled = false`. Each run works on a private copy of the file, so concurrent runs don't affect each other. Changes made during the run are copied back when the container exits.

The file must be a regular file: agentic refuses a symlink there, since the tool can write that directory and could otherwise redirect the copy-back to any host file. Reads and writes also stay inside `$AGENTIC_HOME/tools`, so a tool directory swapped for a symlink can't redirect them either.

Preview the block with `agentic instructions claude` (add `--proxy` to include the network section).

### `[run.proxy]`

> [!WARNING]
> **Beta** - see [Egress proxy](egress-proxy.md).

| Key             | Type   | Description                                                                                       | CLI flag                 | Default     |
| --------------- | ------ | ------------------------------------------------------------------------------------------------- | ------------------------ | ----------- |
| `enabled`       | bool   | Route egress through the allowlist proxy                                                          | `--proxy` / `--no-proxy` | `false`     |
| `mode`          | string | `"enforce"` blocks hosts not on the allowlist, `"monitor"` only logs them. `"monitor"` turns the proxy on unless `enabled = false`. | `--proxy-monitor`        | `"enforce"` |
| `allowed_hosts` | list   | Hosts added to the tool's baseline. Exact (`"api.github.com"`), or `.`/`*.` prefix for a domain and its subdomains (`".github.com"`). | -                        | -           |

### `[[run.proxy.credentials]]`

> [!WARNING]
> **Beta** - see [Credential injection](egress-proxy.md#credential-injection).

| Key      | Type   | Description                                                                                              |
| -------- | ------ | -------------------------------------------------------------------------------------------------------- |
| `preset` | string | `anthropic`, `openai` or `github`. Can't be combined with `hosts`, `header` or `format`.                 |
| `hosts`  | list   | Exact hostnames or IPs, no wildcards. Required without `preset`. A host may appear in only one entry.    |
| `header` | string | Header to set, e.g. `"Authorization"`. Required without `preset`. Hop-by-hop, `Host` and `Content-Length` are refused. |
| `format` | string | Wraps the secret, with exactly one `%s`, e.g. `"Bearer %s"`. Default: the bare secret.                   |
| `env`    | list   | Tool env vars set to the placeholder. Replaces a preset's own list.                                      |
| `secret` | string | Required. Host file with the secret: an absolute path, or one starting with `~`/`$HOME`. One line, at most 64 KiB. Other sources (`keychain:`) are reserved. |

| Preset      | Hosts and header                                                                                                      | Env                        |
| ----------- | --------------------------------------------------------------------------------------------------------------------- | -------------------------- |
| `anthropic` | `api.anthropic.com`: `X-Api-Key: <secret>`                                                                            | `ANTHROPIC_API_KEY`        |
| `openai`    | `api.openai.com`: `Authorization: Bearer <secret>`                                                                    | `OPENAI_API_KEY`           |
| `github`    | `api.github.com`: `Authorization: Bearer <secret>`; `github.com`: basic auth as `x-access-token`, for git over HTTPS | `GITHUB_TOKEN`, `GH_TOKEN` |

```toml
[[run.proxy.credentials]]
preset = "anthropic"
secret = "~/.secrets/anthropic_key"

[[run.proxy.credentials]]
hosts = ["api.example.com"]
header = "Authorization"
format = "Token %s"
env = ["EXAMPLE_TOKEN"]
secret = "~/.secrets/example_token"
```

### `[run.dind]`

> [!WARNING]
> **Beta** - see [Docker-in-Docker](docker-in-docker.md).

| Key          | Type   | Description                                                             | CLI flag               | Default      |
| ------------ | ------ | ----------------------------------------------------------------------- | ---------------------- | ------------ |
| `enabled`    | bool   | Start a per-run rootless Docker sidecar. Needs `--base docker`.         | `--dind` / `--no-dind` | `false`      |
| `pids_limit` | string | Sidecar PID limit                                                       | `--dind-pids-limit`    | tool's value |
| `cpus`       | string | Sidecar CPU limit                                                       | `--dind-cpus`          | tool's value |
| `memory`     | string | Sidecar memory limit (e.g. `"8g"`)                                      | `--dind-memory`        | tool's value |

### `[[marketplaces]]`

Git-based plugin marketplaces to sync on the host and mount read-only into tool containers. See [Marketplaces](usage.md#marketplaces) for how syncing and cleanup work.

| Key     | Type   | Description                                                                                     | Default               |
| ------- | ------ | ----------------------------------------------------------------------------------------------- | --------------------- |
| `name`  | string | Local name, must match `^[a-zA-Z0-9._-]+$`. Used as the mount path `~/marketplaces/<name>`.     | -                     |
| `url`   | string | Any URL `git clone` accepts                                                                     | -                     |
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

## Merge semantics

The file closest to the filesystem root is the _outermost_. The file in `$PWD` is the _innermost_.

- **List keys** (`bases`, `apt_packages`, `custom_installs`, `extra_mounts`, `read_only_mounts`, `secrets`, `env`, `proxy.allowed_hosts`, `proxy.credentials`, `marketplaces`) add up, outermost first.
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
| `pids_limit`, `cpus`, `memory` | flag, then the innermost file, then `1024` / `4` / `4g`                                  |
| `[run.dind]` limits            | `--dind-*` flag, then `[run.dind]`, then the tool's resolved value                       |
| `docker_context`               | `--docker-context`, then the innermost file, then `agentic.json`, then the Docker CLI's own choice (including `DOCKER_CONTEXT`) |

List settings combine instead:

| Setting                   | How it combines                                                                     |
| ------------------------- | ----------------------------------------------------------------------------------- |
| `bases`, `apt_packages`   | Files (outermost first), then `--base`/`--apt`. Duplicates removed.                 |
| `extra_mounts`, `secrets` | Files and `-v`/`-s` combined                                                        |
| `read_only_mounts`        | `--read-only-mount` first, then files                                               |
| `env`                     | Files, then `-e`. The last value for a key wins. Some names are [reserved](usage.md#environment-variables). |

`--base-exact` and `--apt-exact` replace the list instead of adding to it. They ignore config files and, during `agentic update`, the list the image was built with. `--base-exact=` or `--apt-exact=` (empty) means none at all. Each can't be combined with its non-exact flag.

## Mount variable expansion

Mount strings (`extra_mounts`, `read_only_mounts`, `-v`, `--read-only-mount`) expand these placeholders at runtime. The `${VAR}` form works too.

| Placeholder       | Side of `:`       | Expands to                                 |
| ----------------- | ----------------- | ------------------------------------------ |
| `~`, `$HOME`      | host (left)       | Your home directory                        |
| `$TOOL_HOME`      | host (left)       | Agentic data directory (e.g. `~/.agentic`) |
| `$PWD`            | host (left)       | Current directory, mounted as `/workspace` |
| `$CONTAINER_HOME` | container (right) | Container home (e.g. `/home/claude`)       |

Single-quote them so your shell doesn't expand them first:

```bash
agentic run -v '$TOOL_HOME/custom:$CONTAINER_HOME/.custom:rw' claude
```

## Mount safety

Before a run starts, agentic checks every bind mount and secret:

- A mount that would expose `$AGENTIC_HOME/agentic.json` (the agentic home or a parent, such as `~`) is refused, since that file records trusted directories and approvals.
- A path through a symlink inside the workspace is refused, so a planted `.git -> ~/.ssh` can't be mounted via `read_only_mounts = [".git"]`. Mount the real path instead.
