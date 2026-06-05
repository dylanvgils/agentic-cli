# Configuration

Agentic is configured through three layers, applied in order of increasing specificity: `agentic.json` global file, `.agenticrc.toml` project files, environment variables, and CLI flags. List-type settings accumulate across all layers; scalar settings use the most specific value.

## `agentic.json` (global config)

Stored in `$AGENTIC_HOME/agentic.json` (default: `~/.agentic/agentic.json`). This file holds machine-level settings that apply to all projects. Edit it directly with any text editor.

| Key            | Type   | Description                                                                      | CLI flag      |
| -------------- | ------ | -------------------------------------------------------------------------------- | ------------- |
| `trusted_dirs` | list   | Directories trusted to run tools from without an interactive prompt              | `--trust-dir` |
| `registry`     | scalar | Registry prefix for base image pulls (e.g. `myregistry.example.com`). See below. | `--registry`  |

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

| Key            | Type   | Description                                                                                                                                                        | CLI flag       | Env var                | Default |
| -------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------- | ---------------------- | ------- |
| `extra_mounts` | list   | Extra mounts passed to `docker run`. Bind: `host/path:container/path`. Named volume: `name:container/path`. Supports `~`, `$HOME`, `$TOOL_HOME`, `$CONTAINER_HOME` | `-v`           | `AGENTIC_EXTRA_MOUNTS` | -       |
| `secrets`      | list   | Files to mount read-only at `/run/secrets/<name>`. Format: `name:/path/to/file`. Supports `~`, `$HOME`                                                             | `-s`           | `AGENTIC_SECRETS`      | -       |
| `pids_limit`   | string | Container PID limit (e.g. `"1024"`)                                                                                                                                | `--pids-limit` | `AGENTIC_PIDS_LIMIT`   | `1024`  |
| `cpus`         | string | Container CPU limit (e.g. `"4"`)                                                                                                                                   | `--cpus`       | `AGENTIC_CPUS`         | `4`     |
| `memory`       | string | Container memory limit (e.g. `"8g"`)                                                                                                                               | `--memory`     | `AGENTIC_MEMORY`       | `4g`    |

### Merge semantics

When multiple `.agenticrc.toml` files are found, they are merged. The walk starts at `$PWD` and moves upward, so the file closest to the root is the _outermost_ and the file in `$PWD` is the _innermost_.

- **List keys** (`bases`, `apt_packages`, `extra_mounts`, `secrets`): values from all levels accumulate, outermost first.
- **Scalar keys** (`pids_limit`, `cpus`, `memory`, `namespace`): the innermost (child) value wins; outer files fill in any keys the inner file does not set.
- **`versions` table**: each layer name is resolved independently — innermost value wins per key, so a child can pin `java` without affecting `node` inherited from a parent.

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

Duplicates are removed while preserving order.

### `bases`

Extra runtime layers accumulate across RC files and the `--base` flag:

1. `.agenticrc.toml` files (outermost first)
2. `--base` flag (appended, deduplicated)

`AGENTIC_BASE_OVERRIDE` is a full override — when set it replaces all RC and flag values.

### `versions`

Per-layer version resolution (highest to lowest priority):

1. `--<layer>` flag (e.g. `--java 17`) or `AGENTIC_<LAYER>_VERSION` env var
2. `.agenticrc.toml` `[build.versions]` — innermost value wins per key
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

## Inspecting the merged config

Run `agentic config` to see the merged result of all active `.agenticrc.toml` files and environment variables for the current directory:

```
agentic config
```
