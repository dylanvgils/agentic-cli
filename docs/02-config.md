# Configuration

Agentic is configured through three layers, applied in order of increasing specificity: `agentic.json` global file, `.agenticrc.toml` project files, environment variables, and CLI flags. List-type settings accumulate across all layers; scalar settings use the most specific value.

## `agentic.json` (global config)

Stored in `$AGENTIC_HOME/agentic.json` (default: `~/.agentic/agentic.json`). This file holds machine-level settings that apply to all projects. Edit it directly with any text editor.

| Key                        | Type   | Description                                                                                | CLI flag      |
| -------------------------- | ------ | ------------------------------------------------------------------------------------------ | ------------- |
| `trusted_dirs`             | list   | Directories trusted to run tools from without an interactive prompt                        | `--trust-dir` |
| `registry`                 | scalar | Registry prefix for base image pulls (e.g. `myregistry.example.com`). See below.           | `--registry`  |
| `proxy_log_retention_days` | scalar | Days to keep egress proxy access logs before they're pruned automatically. Default: `3`.   | -             |
| `last_update_check`        | scalar | Timestamp of the last automatic update check. Managed automatically - do not edit by hand. | -             |

### Registry proxy

If your environment requires pulling Docker Hub images through a registry proxy (e.g. Harbor, Nexus, Artifactory, AWS ECR pull-through cache), set the `registry` field:

```json
{
  "registry": "myregistry.example.com"
}
```

Agentic prefixes all base image names with this value at build time, routing pulls through the proxy. Authentication is out of scope - configure it once with `docker login myregistry.example.com`.

The `--registry` flag overrides the `agentic.json` value for a single build:

```bash
agentic build claude --registry myregistry.example.com
```

Run `agentic config` to see the active registry setting.

## `.agenticrc.toml` files

Place a `.agenticrc.toml` file in any directory to apply settings when `agentic` is run from that directory or any subdirectory. `agentic` walks up from `$PWD` collecting every `.agenticrc.toml` it finds, stopping when it hits a file with `root = true` or the filesystem root.

### File format

Standard [TOML](https://toml.io). Comments start with `#`. List keys use TOML arrays. Build-time and runtime settings live in separate `[build]` and `[run]` sections; `root` and `namespace` are top-level keys.

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
pids_limit = "2048"
```

### Keys

**Top-level**

| Key         | Type   | Description                                                                                                            | Env var             | Default   |
| ----------- | ------ | ---------------------------------------------------------------------------------------------------------------------- | ------------------- | --------- |
| `root`      | bool   | Stop the upward directory walk at this file                                                                            | -                   | -         |
| `namespace` | string | Image namespace. Images are named `<namespace>-<tool>` (e.g. `myproject-claude`). Allows multiple image sets per tool. | `AGENTIC_NAMESPACE` | `agentic` |

**`[build]` section** - applied at `agentic build` / `agentic update` time

| Key            | Type       | Description                                                                                                                      | CLI flag    | Env var                   | Default |
| -------------- | ---------- | -------------------------------------------------------------------------------------------------------------------------------- | ----------- | ------------------------- | ------- |
| `bases`        | list       | Extra runtime layers to add on top of the node base (e.g. `["java", "dotnet"]`). Accumulates across RC layers and with `--base`. | `--base`    | -                         | -       |
| `apt_packages` | list       | Extra Debian packages to install in the base image. Accumulates across RC layers and with `--apt`.                               | `--apt`     | `AGENTIC_APT_PACKAGES`    | -       |
| `versions`     | TOML table | Per-layer version pins. Written as `[build.versions]` with `node`, `java`, `dotnet`, or `go` keys. Innermost value wins per key. | `--<layer>` | `AGENTIC_<LAYER>_VERSION` | -       |

**`[run]` section** - applied at `agentic run` time

| Key            | Type   | Description                                                                                                                                                                                    | CLI flag       | Env var                | Default |
| -------------- | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------- | ---------------------- | ------- |
| `extra_mounts` | list   | Extra mounts passed to `docker run`. Bind: `host/path:container/path`. Named volume: `name:container/path`. Supports `~`, `$HOME`, `$TOOL_HOME`, `$CONTAINER_HOME`                             | `-v`           | `AGENTIC_EXTRA_MOUNTS` | -       |
| `secrets`      | list   | Files to mount read-only into the container. Format: `name:/path/to/file[:/container/path]`. Defaults to `/run/secrets/<name>`. Supports `~`, `$HOME`, `$CONTAINER_HOME` (container path only) | `-s`           | `AGENTIC_SECRETS`      | -       |
| `pids_limit`   | string | Container PID limit (e.g. `"1024"`)                                                                                                                                                            | `--pids-limit` | `AGENTIC_PIDS_LIMIT`   | `1024`  |
| `cpus`         | string | Container CPU limit (e.g. `"4"`)                                                                                                                                                               | `--cpus`       | `AGENTIC_CPUS`         | `4`     |
| `memory`       | string | Container memory limit (e.g. `"8g"`)                                                                                                                                                           | `--memory`     | `AGENTIC_MEMORY`       | `4g`    |

**`[run.proxy]` section** - egress allowlist proxy

| Key             | Type | Description                                                                                                                                                                                        | CLI flag                 | Default |
| --------------- | ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------ | ------- |
| `enabled`       | bool | Route the tool's egress through the allowlist proxy. `enabled` is a pointer internally so an inner config can explicitly disable a proxy enabled by an outer one.                                  | `--proxy` / `--no-proxy` | `false` |
| `allowed_hosts` | list | Extra hosts to permit, merged on top of the tool's baseline. Exact match (e.g. `"api.github.com"`), or a leading-dot / `*.` entry to match a domain and all its subdomains (e.g. `".github.com"`). | -                        | -       |

When the proxy is enabled the tool container loses direct internet access. It runs on a per-run internal Docker network and reaches the outside only through a proxy sidecar that enforces the allowlist. Blocked hosts are printed at the end of the run; every connection attempt is logged as JSON lines under `$AGENTIC_HOME/proxy/`.

Each proxy-enabled run prunes its own access logs before starting, deleting any older than a retention window (default 3 days). This is a host-level setting, not a per-project one, so it isn't part of `.agenticrc.toml`: set `proxy_log_retention_days` in `agentic.json` (see the table above) to change it. To wipe every log regardless of age, run `agentic proxy clean --logs`.

Each tool ships a baseline allowlist; `allowed_hosts` values are merged on top of it. The proxy image is built on demand the first time you run with `--proxy`, or explicitly via `agentic proxy build`/`agentic proxy update` (see [Development](05-development.md#building-the-proxy-image-locally)).

| Tool       | Baseline host        | Purpose                             |
| ---------- | -------------------- | ----------------------------------- |
| `claude`   | `.anthropic.com`     | Claude API and telemetry subdomains |
| `claude`   | `.claude.ai`         | installer and asset downloads       |
| `claude`   | `.claude.com`        | OAuth/login flow                    |
| `copilot`  | `.githubcopilot.com` | Copilot API and subdomains          |
| `copilot`  | `api.github.com`     | GitHub API used for authentication  |
| `opencode` | `opencode.ai`        | OpenCode auth and update checks     |

OpenCode is multi-provider, so only its own auth/update host is included by default - add your chosen model-provider hosts via `allowed_hosts`.

`agentic config` shows the resolved `proxy.enabled` and `proxy.allowed_hosts` values for the current directory, tagged with the `.agenticrc.toml` that set them (the tool's baseline hosts aren't part of this output - they're fixed per tool, not configurable).

```toml
[run.proxy]
enabled = true
allowed_hosts = [
  "registry.npmjs.org",
  ".github.com",
]
```

### Merge semantics

When multiple `.agenticrc.toml` files are found, they are merged. The walk starts at `$PWD` and moves upward, so the file closest to the root is the _outermost_ and the file in `$PWD` is the _innermost_.

- **List keys** (`bases`, `apt_packages`, `extra_mounts`, `secrets`, `proxy.allowed_hosts`): values from all levels accumulate, outermost first.
- **Scalar keys** (`pids_limit`, `cpus`, `memory`, `namespace`): the innermost (child) value wins; outer files fill in any keys the inner file does not set.
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

These also accumulate, but their env vars (`AGENTIC_EXTRA_MOUNTS`, `AGENTIC_SECRETS`) and RC values are each collected independently and combined at runtime.

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

### Scalar settings (`pids_limit`, `cpus`, `memory`)

Resolution priority (highest to lowest):

1. CLI flag (`--pids-limit`, `--cpus`, `--memory`) on `agentic run`
2. `.agenticrc.toml` - innermost (child) value wins
3. Environment variable (`AGENTIC_PIDS_LIMIT`, `AGENTIC_CPUS`, `AGENTIC_MEMORY`)
4. Built-in default (`1024`, `4`, `4g`)

## Using `root = true`

`root = true` marks a boundary in the directory walk. It is useful for monorepos where you want a single shared config at the repo root and per-project configs in subdirectories, without accidentally picking up configs from outside the repo:

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

Several placeholders are expanded in mount strings (`extra_mounts`, `AGENTIC_EXTRA_MOUNTS`, `-v`) at runtime, so paths don't have to be hardcoded per machine or per tool:

| Placeholder         | Side of `:`       | Expands to                                     |
| ------------------- | ----------------- | ---------------------------------------------- |
| `~`                 | host (left)       | Your home directory                            |
| `$HOME`             | host (left)       | Same as above                                  |
| `${HOME}`           | host (left)       | Same as above                                  |
| `$TOOL_HOME`        | host (left)       | Agentic data directory (e.g. `~/.agentic`)     |
| `${TOOL_HOME}`      | host (left)       | Same as above                                  |
| `$CONTAINER_HOME`   | container (right) | Container home directory (e.g. `/home/claude`) |
| `${CONTAINER_HOME}` | container (right) | Same as above                                  |

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
