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

Standard [TOML](https://toml.io). Comments start with `#`. List keys use TOML arrays.

```toml
# .agenticrc.toml
root = true

apt_packages = ["make", "gcc", "jq"]

extra_mounts = ["maven:$CONTAINER_HOME/.m2"]

pids_limit = "2048"
```

### Keys

| Key            | Type   | Description                                                                                                                                                        | CLI flag       | Env var                | Default   |
| -------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------- | ---------------------- | --------- |
| `root`         | bool   | Stop the upward directory walk at this file                                                                                                                        | -              | -                      | -         |
| `namespace`    | string | Image namespace. Images are named `<namespace>-<tool>` (e.g. `myproject-claude`). Allows multiple image sets per tool.                                             | `--namespace`  | `AGENTIC_NAMESPACE`    | `agentic` |
| `apt_packages` | list   | Extra Debian packages to install in the base image at build time                                                                                                   | -              | `AGENTIC_APT_PACKAGES` | -         |
| `extra_mounts` | list   | Extra mounts passed to `docker run`. Bind: `host/path:container/path`. Named volume: `name:container/path`. Supports `~`, `$HOME`, `$TOOL_HOME`, `$CONTAINER_HOME` | -              | `AGENTIC_EXTRA_MOUNTS` | -         |
| `secrets`      | list   | Files to mount read-only at `/run/secrets/<name>`. Format: `name:/path/to/file`. Supports `~`, `$HOME`                                                             | -              | `AGENTIC_SECRETS`      | -         |
| `pids_limit`   | string | Container PID limit (e.g. `"1024"`)                                                                                                                                | `--pids-limit` | `AGENTIC_PIDS_LIMIT`   | `1024`    |
| `cpus`         | string | Container CPU limit (e.g. `"4"`)                                                                                                                                   | `--cpus`       | `AGENTIC_CPUS`         | `4`       |
| `memory`       | string | Container memory limit (e.g. `"8g"`)                                                                                                                               | `--memory`     | `AGENTIC_MEMORY`       | `4g`      |

### Merge semantics

When multiple `.agenticrc.toml` files are found, they are merged. The walk starts at `$PWD` and moves upward, so the file closest to the root is the _outermost_ and the file in `$PWD` is the _innermost_.

- **List keys** (`apt_packages`, `extra_mounts`, `secrets`): values from all levels accumulate, outermost first.
- **Scalar keys** (`pids_limit`, `cpus`, `memory`, `namespace`): the innermost (child) value wins; outer files fill in any keys the inner file does not set.

```
~/projects/.agenticrc.toml              ← outermost (root=true stops the walk here)
~/projects/my-project/.agenticrc.toml  ← innermost ($PWD)
```

Given these two files:

```toml
# ~/projects/.agenticrc.toml
root = true
apt_packages = ["make"]
cpus = "4"
```

```toml
# ~/projects/my-project/.agenticrc.toml
apt_packages = ["gcc"]
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
apt_packages = ["make"]
```

Then `agentic build claude --base java` creates `java-app-claude`, while the default `agentic-claude` remains untouched.

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
secrets = ["gh-token:~/.secrets/gh_token"]
apt_packages = ["make"]
```

```toml
# ~/projects/my-project/.agenticrc.toml - project-specific additions
extra_mounts = ["maven:$CONTAINER_HOME/.m2"]
apt_packages = ["gcc"]
cpus = "8"
```

Running `agentic` from `~/projects/my-project` merges both files and stops; `~/projects` is not traversed further even if a `.agenticrc.toml` exists above it.

## Inspecting the merged config

Run `agentic config` to see the merged result of all active `.agenticrc.toml` files and environment variables for the current directory:

```
agentic config
```
