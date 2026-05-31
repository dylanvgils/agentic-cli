# Configuration

Agentic is configured through three layers, applied in order of increasing specificity: `.agenticrc` project files, environment variables, and CLI flags. List-type settings accumulate across all layers; scalar settings use the most specific value.

## `.agenticrc` files

Place a `.agenticrc` file in any directory to apply settings when `agentic` is run from that directory or any subdirectory. `agentic` walks up from `$PWD` collecting every `.agenticrc` it finds, stopping when it hits a file with `root=true` or the filesystem root.

### File format

Plain `key=value` pairs, one per line. Comments start with `#`. Values may be quoted (single or double) or unquoted.

```sh
# .agenticrc
root=true

apt_packages=make,gcc   # comma-separated
apt_packages=jq         # or one entry per line - both styles work

extra_mounts=maven:$CONTAINER_HOME/.m2

pids_limit=2048
```

### Keys

| Key            | Type   | Description                                                                                                                                                        | CLI flag       | Env var                | Default |
| -------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------- | ---------------------- | ------- |
| `root`         | scalar | Stop the upward directory walk at this file (`true`/`false`)                                                                                                       | -              | -                      | -       |
| `apt_packages` | list   | Extra Debian packages to install in the base image at build time                                                                                                   | -              | `AGENTIC_APT_PACKAGES` | -       |
| `extra_mounts` | list   | Extra mounts passed to `docker run`. Bind: `host/path:container/path`. Named volume: `name:container/path`. Supports `~`, `$HOME`, `$TOOL_HOME`, `$CONTAINER_HOME` | -              | `AGENTIC_EXTRA_MOUNTS` | -       |
| `secrets`      | list   | Files to mount read-only at `/run/secrets/<name>`. Format: `name:/path/to/file`. Supports `~`, `$HOME`                                                             | -              | `AGENTIC_SECRETS`      | -       |
| `pids_limit`   | scalar | Container PID limit                                                                                                                                                | `--pids-limit` | `AGENTIC_PIDS_LIMIT`   | `1024`  |
| `cpus`         | scalar | Container CPU limit                                                                                                                                                | `--cpus`       | `AGENTIC_CPUS`         | `4`     |
| `memory`       | scalar | Container memory limit                                                                                                                                             | `--memory`     | `AGENTIC_MEMORY`       | `4g`    |

### Merge semantics

When multiple `.agenticrc` files are found, they are merged. The walk starts at `$PWD` and moves upward, so the file closest to the root is the _outermost_ and the file in `$PWD` is the _innermost_.

- **List keys** (`apt_packages`, `extra_mounts`, `secrets`): values from all levels accumulate, outermost first.
- **Scalar keys** (`pids_limit`, `cpus`, `memory`): the innermost (child) value wins; outer files fill in any keys the inner file does not set.

```
~/projects/.agenticrc       ← outermost (root=true stops the walk here)
~/projects/my-project/.agenticrc  ← innermost ($PWD)
```

Given these two files:

```sh
# ~/projects/.agenticrc
root=true
apt_packages=make
cpus=4

# ~/projects/my-project/.agenticrc
apt_packages=gcc
cpus=8
```

The effective configuration is `apt_packages=[make, gcc]` and `cpus=8` (child wins for scalars).

## Precedence

### `apt_packages`

Packages accumulate across all three sources in this order:

1. `.agenticrc` files (outermost first)
2. `AGENTIC_APT_PACKAGES` environment variable (comma-separated)
3. `--apt` flag

Duplicates are removed while preserving order.

### `extra_mounts` and `secrets`

These also accumulate, but their env vars (`AGENTIC_EXTRA_MOUNTS`, `AGENTIC_SECRETS`) and RC values are each collected independently and combined at runtime.

### Scalar settings (`pids_limit`, `cpus`, `memory`)

Resolution priority (highest to lowest):

1. CLI flag (`--pids-limit`, `--cpus`, `--memory`) on `agentic run`
2. `.agenticrc` - innermost (child) value wins
3. Environment variable (`AGENTIC_PIDS_LIMIT`, `AGENTIC_CPUS`, `AGENTIC_MEMORY`)
4. Built-in default (`1024`, `4`, `4g`)

## Using `root=true`

`root=true` marks a boundary in the directory walk. It is useful for monorepos where you want a single shared config at the repo root and per-project configs in subdirectories, without accidentally picking up configs from outside the repo:

```sh
# ~/projects/.agenticrc - shared config for all projects
root=true
secrets=gh-token:~/.secrets/gh_token
apt_packages=make

# ~/projects/my-project/.agenticrc - project-specific additions
extra_mounts=maven:$CONTAINER_HOME/.m2
apt_packages=gcc
cpus=8
```

Running `agentic` from `~/projects/my-project` merges both files and stops; `~/projects` is not traversed further even if a `.agenticrc` exists above it.

## Inspecting the merged config

Run `agentic config` to see the merged result of all active `.agenticrc` files and environment variables for the current directory:

```
agentic config
```
