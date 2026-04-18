# Agentic CLI

CLI for running agentic coding tools in sandboxed Docker containers.

## 📖 Overview

Each tool runs in an isolated, read-only container with only the minimal mounts it needs - your workspace and its own config directory. No root, no extra capabilities, no leftovers when done.

## 📋 Requirements

- Docker
- Git

## 🚀 Installation

Clone the repo and run the install script:

```bash
git clone https://github.com/dylanvgils/agentic-cli.git
cd agentic-cli
./install.sh
```

To uninstall:

```bash
./uninstall.sh
```

Then build the image(s) you need:

```bash
agentic build                        # Build all tools
agentic build claude                 # Claude agent only
agentic build copilot                # GitHub Copilot agent only
agentic build opencode               # OpenCode agent only
agentic build claude --base java     # Claude with Java runtime added
agentic build claude --no-cache      # Force a fully fresh build
```

To remove all containers and images created by this project:

```bash
agentic clean
```

## 🛠️ Usage

```bash
agentic <command> [args...]
```

### Commands

| Command                                                                                                                    | Description                                                                                 |
| -------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| `build [tool] [--base <extras>] [--no-cache] [--node <version>] [--java <version>] [--dotnet <version>] [--go <version>]`  | Build tool image(s). Builds all tools if unspecified                                        |
| `update [tool] [--base <extras>] [--no-cache] [--node <version>] [--java <version>] [--dotnet <version>] [--go <version>]` | Update tool image(s) to latest version. Skips unbuilt tools when unspecified                |
| `clean [tool]`                                                                                                             | Remove tool image(s). Cleans all tools + base if unspecified                                |
| `inspect [tool]`                                                                                                           | Show image info (version, base layers, build date, size). Inspects all tools if unspecified |
| `volumes <create\|list\|ls\|remove\|rm> [name]`                                                                            | Manage named Docker volumes created by agentic                                              |
| `completion [shell]`                                                                                                       | Print shell completion script (`bash` or `zsh`, defaults to `zsh`)                          |
| `aliases [shell]`                                                                                                          | Print shell alias definitions for tools (`bash` or `zsh`, defaults to `zsh`)                |
| `help [command]`                                                                                                           | Show help for a command (`run` for tool run options). Shows overview if unspecified         |
| `<tool> [args]`                                                                                                            | Run a tool in a sandboxed Docker container                                                  |
| `<tool> -- <cmd> [args]`                                                                                                   | Override the entrypoint and run a shell command directly                                    |

Run tool commands from within a git repository. The current directory is mounted as `/workspace` inside the container.

### Tools

| Tool       | Description        |
| ---------- | ------------------ |
| `claude`   | Claude Code CLI    |
| `copilot`  | GitHub Copilot CLI |
| `opencode` | OpenCode CLI       |

### Examples

```bash
# Build images
agentic build
agentic build copilot

# Build with an extra runtime on top of node
agentic build claude --base java

# Force a fully fresh build (bypasses Docker layer cache)
agentic build claude --no-cache
agentic build claude --base java --no-cache

# Build with dotnet runtime
agentic build claude --base dotnet
agentic build claude --base dotnet --dotnet 9

# Pin specific runtime versions
agentic build --node 22
agentic build claude --base java --java 17
agentic build claude --base java --node 22 --java 17 --no-cache
agentic build claude --base dotnet --dotnet 9 --no-cache

# Clean images
agentic clean
agentic clean claude

# Inspect built images
agentic inspect
agentic inspect claude

# Update to latest version (only rebuilds the tool step, base layers stay cached)
agentic update
agentic update claude
agentic update claude --base java
# Force a fully fresh update (also rebuilds base layers)
agentic update claude --no-cache

# Run a specific tool
agentic claude
agentic copilot
agentic opencode

# Run a shell command instead of the tool entrypoint
agentic claude -- bash
agentic claude -- ls /workspace

# Mount named Docker volumes (auto-created on first use)
agentic -v 'maven:$CONTAINER_HOME/.m2' claude
agentic -v 'maven:$CONTAINER_HOME/.m2' -v 'gradle:$CONTAINER_HOME/.gradle' claude

# Mount bind-mount volumes (host paths)
agentic -v '~/.m2:$CONTAINER_HOME/.m2' claude
agentic -v '~/.m2:$CONTAINER_HOME/.m2' -v '~/.gradle:$CONTAINER_HOME/.gradle' claude

# Override the tool home directory
agentic --home /opt/agentic claude

# Run with no arguments if a default tool is configured
agentic

# Print completion script
agentic completion        # zsh (default)
agentic completion bash
```

## 🔁 Shell completion

Tab completion is available for bash and zsh. Add one of the following to your shell config to activate it:

```bash
# zsh - add to ~/.zshrc
source <(agentic completion)

# bash - add to ~/.bashrc
source <(agentic completion bash)
```

Completions cover all commands (`build`, `update`, `clean`, `inspect`, `completion`, `aliases`, `help`), tool names, and command-specific flags (`--base`, `--no-cache`, `--help`). Tool names are discovered dynamically at completion time, so new tools are picked up automatically without regenerating the script.

## 🔗 Shell aliases

Shell aliases let you run tools directly (e.g., `copilot` instead of `agentic copilot`). Add one of the following to your shell config to activate them:

```bash
# zsh - add to ~/.zshrc
source <(agentic aliases)

# bash - add to ~/.bashrc
source <(agentic aliases bash)
```

Only tools with a built image produce an alias, so sourcing the output never creates broken aliases for uninstalled tools.

## 🧱 Base images

Node is always the root layer. The `--base` flag adds extra runtimes on top of it:

```
node (agentic-base)
  ├── java   (agentic-base-java)   ← added with --base java
  ├── dotnet (agentic-base-dotnet) ← added with --base dotnet
  └── go     (agentic-base-go)     ← added with --base go
        └── tool image
```

| Flag                                 | Result             |
| ------------------------------------ | ------------------ |
| _(none)_                             | node only (v24)    |
| `--base java`                        | node v24 + Java 21 |
| `--base dotnet`                      | node v24 + .NET 10 |
| `--base go`                          | node v24 + Go 1.26 |
| `--node 22`                          | node v22 only      |
| `--base java --java 17`              | node v24 + Java 17 |
| `--base dotnet --dotnet 9`           | node v24 + .NET 9  |
| `--base go --go 1.23`                | node v24 + Go 1.23 |
| `--node 22 --base java --java 17`    | node v22 + Java 17 |
| `--node 22 --base dotnet --dotnet 9` | node v22 + .NET 9  |

Both tools default to node only. Use `--base` to add extra runtimes at build time.

Version defaults live in the Dockerfiles (`NODE_VERSION=24`, `JAVA_VERSION=21`, `DOTNET_VERSION=10`, `GO_VERSION=1.26.2`). Override them per-build with `--node`/`--java`/`--dotnet`/`--go`, or set `AGENTIC_NODE_VERSION`/`AGENTIC_JAVA_VERSION`/`AGENTIC_DOTNET_VERSION`/`AGENTIC_GO_VERSION` in your shell config for persistent defaults.

Adding a new runtime is a matter of dropping a `Dockerfile` into `shared/base/<name>/` - it will be picked up automatically by `--base <name>`.

The final tool image is labeled with the base layers, build timestamp, and installed tool version:

```bash
docker inspect agentic-claude --format '{{ index .Config.Labels "agentic.base" }}'
# → node,java

docker inspect agentic-claude --format '{{ index .Config.Labels "agentic.built" }}'
# → 2026-04-05T14:30:00Z

docker inspect agentic-claude --format '{{ index .Config.Labels "agentic.tool.version" }}'
# → Claude Code 1.2.3
```

Use `agentic inspect` for a formatted summary of all of the above.

## 📦 Named Docker volumes

The `-v` flag and `AGENTIC_EXTRA_MOUNTS` support both bind mounts (host paths) and named Docker volumes. Named volumes are created automatically on first use and persist across container runs — no host path required.

Use a volume name (no leading `/`) as the left side of the mount spec:

```bash
# Named Docker volumes (created automatically, managed by Docker)
agentic -v 'maven:$CONTAINER_HOME/.m2' claude
agentic -v 'maven:$CONTAINER_HOME/.m2' -v 'gradle:$CONTAINER_HOME/.gradle' claude

# Bind mounts (path on the host)
agentic -v '~/.m2:$CONTAINER_HOME/.m2' claude
```

For persistent global config, set `AGENTIC_EXTRA_MOUNTS` in your shell:

```bash
export AGENTIC_EXTRA_MOUNTS='maven:$CONTAINER_HOME/.m2,gradle:$CONTAINER_HOME/.gradle'
```

For per-project control, use a [`.agenticrc` project config file](#per-project-configuration):

```sh
# .agenticrc
EXTRA_MOUNTS=maven:$CONTAINER_HOME/.m2,gradle:$CONTAINER_HOME/.gradle
```

### Managing volumes

Use `agentic volumes` to inspect and clean up agentic-managed volumes:

```bash
agentic volumes create maven      # Create a named volume
agentic volumes list              # List all agentic-managed volumes (alias: ls)
agentic volumes remove maven      # Remove a specific volume (alias: rm)
agentic volumes remove            # Remove all agentic-managed volumes
```

## ☕ Java build tools

Maven and Gradle are **not** included in the Java base image. Instead, use the wrappers that come with your project (`mvnw` / `gradlew`). Wrappers are committed to the repo and download the exact build tool version the project requires on first run - this avoids version mismatches and keeps the image lean.

To generate a wrapper if your project doesn't have one yet:

```bash
# Maven
mvn wrapper:wrapper

# Gradle
gradle wrapper
```

Use named volumes to persist the download cache across container runs:

```bash
agentic -v 'maven:$CONTAINER_HOME/.m2' -v 'gradle:$CONTAINER_HOME/.gradle' claude
```

Or add to `.agenticrc` in the repo root so the whole team picks it up:

```sh
# .agenticrc
EXTRA_MOUNTS=maven:$CONTAINER_HOME/.m2,gradle:$CONTAINER_HOME/.gradle
```

## ⚙️ Configuration

All configuration is done through environment variables, which can be set in your shell config (`.zshrc`, `.bashrc`, etc.).

| Variable                 | Description                                                                                                                                           | Default                   |
| ------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------- |
| `AGENTIC_DEFAULT_TOOL`   | Default tool when none is specified                                                                                                                   | -                         |
| `AGENTIC_HOME`           | Base directory for tool config and secrets                                                                                                            | `$HOME/.agentic`          |
| `AGENTIC_EXTRA_MOUNTS`   | Comma-separated extra mounts. Bind mount: `host/path:container/path`. Named volume: `name:container/path` (auto-created). Supports `$CONTAINER_HOME`. | -                         |
| `AGENTIC_PIDS_LIMIT`     | Default container PID limit                                                                                                                           | `1024`                    |
| `AGENTIC_CPUS`           | Default container CPU limit                                                                                                                           | `4`                       |
| `AGENTIC_MEMORY`         | Default container memory limit                                                                                                                        | `4g`                      |
| `AGENTIC_NODE_VERSION`   | Node.js version used when building the base node image                                                                                                | `24` (Dockerfile default) |
| `AGENTIC_JAVA_VERSION`   | Java (Temurin JDK) version used when building the java layer                                                                                          | `21` (Dockerfile default) |
| `AGENTIC_DOTNET_VERSION` | .NET version used when building the dotnet layer                                                                                                      | `10` (Dockerfile default) |

### Per-project configuration

Place a `.agenticrc` file in your project root to set project-specific configuration. `agentic` walks up from `$PWD` to find the nearest config file, so it works from any subdirectory.

| Key            | Description                                                                                                                                                | Default | Env var override       |
| -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | ------- | ---------------------- |
| `EXTRA_MOUNTS` | Comma-separated extra mounts. Bind mount: `~/path:container/path`. Named volume: `name:container/path` (auto-created). Supports `~` and `$CONTAINER_HOME`. | -       | `AGENTIC_EXTRA_MOUNTS` |
| `PIDS_LIMIT`   | Container PID limit                                                                                                                                        | `1024`  | `AGENTIC_PIDS_LIMIT`   |
| `CPUS`         | Container CPU limit                                                                                                                                        | `4`     | `AGENTIC_CPUS`         |
| `MEMORY`       | Container memory limit                                                                                                                                     | `4g`    | `AGENTIC_MEMORY`       |

`.agenticrc` values override env var defaults but are superseded by CLI flags. `EXTRA_MOUNTS` is appended to rather than replacing `AGENTIC_EXTRA_MOUNTS`. You can commit `.agenticrc` to the repo so the whole team picks up the right settings automatically.

```sh
# .agenticrc
EXTRA_MOUNTS=maven:$CONTAINER_HOME/.m2,gradle:$CONTAINER_HOME/.gradle
PIDS_LIMIT=2048
CPUS=8
MEMORY=8g
```

### Mount variable substitution

Two placeholders are substituted in mount strings at runtime. Use them so you don't have to hardcode paths that vary per machine or per tool:

| Placeholder         | Side of `:`       | Expands to                                     |
| ------------------- | ----------------- | ---------------------------------------------- |
| `$TOOL_HOME`        | host (left)       | Agentic data directory (e.g. `~/.agentic`)     |
| `${TOOL_HOME}`      | host (left)       | Same as above                                  |
| `$CONTAINER_HOME`   | container (right) | Container home directory (e.g. `/home/claude`) |
| `${CONTAINER_HOME}` | container (right) | Same as above                                  |

Use single quotes (or escape the `$`) so the shell doesn't try to expand the variables before passing them to `agentic`:

```bash
agentic -v '$TOOL_HOME/custom:$CONTAINER_HOME/.custom' claude
export AGENTIC_EXTRA_MOUNTS='~/.m2:$CONTAINER_HOME/.m2,~/.gradle:$CONTAINER_HOME/.gradle'
```

### Example `.zshrc`

```bash
export AGENTIC_DEFAULT_TOOL=claude
# export AGENTIC_HOME="${HOME}/.agentic"   # default; uncomment to override
# export AGENTIC_NODE_VERSION=22   # uncomment to pin Node.js version
# export AGENTIC_JAVA_VERSION=17   # uncomment to pin Java version
# export AGENTIC_DOTNET_VERSION=9  # uncomment to pin .NET version

# Mount Maven and Gradle caches for Java projects (named volumes)
# export AGENTIC_EXTRA_MOUNTS='maven:$CONTAINER_HOME/.m2,gradle:$CONTAINER_HOME/.gradle'
```

## 🏠 Tool home directory

Each tool stores its configuration under `$AGENTIC_HOME`:

| Tool       | Config path                                                  |
| ---------- | ------------------------------------------------------------ |
| `claude`   | `$AGENTIC_HOME/claude/`, `$AGENTIC_HOME/claude/.claude.json` |
| `copilot`  | `$AGENTIC_HOME/copilot/`                                     |
| `opencode` | `$AGENTIC_HOME/opencode/` (data, cache, state)               |

The copilot tool will also use `$HOME/.secrets/copilot_token` if it exists, to reuse the host session token.

## 📁 Repository structure

```
agentic-cli/
├── install.sh                   # Symlink agentic into ~/.local/bin
├── uninstall.sh                 # Remove the symlink
├── bin/
│   └── agentic                  # Main entrypoint: build, clean, and run tools
├── tools/
│   ├── claude/
│   │   ├── build.sh
│   │   ├── clean.sh
│   │   ├── update.sh
│   │   ├── config.sh
│   │   ├── Dockerfile
│   │   ├── entrypoint.sh
│   │   └── run.sh
│   ├── copilot/
│   │   ├── build.sh
│   │   ├── clean.sh
│   │   ├── update.sh
│   │   ├── config.sh
│   │   ├── Dockerfile
│   │   ├── entrypoint.sh
│   │   └── run.sh
│   └── opencode/
│       ├── build.sh
│       ├── clean.sh
│       ├── update.sh
│       ├── config.sh
│       ├── Dockerfile
│       ├── entrypoint.sh
│       └── run.sh
└── shared/
    ├── config.sh                # Shared config (TOOL_HOME)
    ├── base/
    │   ├── node/Dockerfile      # Base Node.js image (root layer)
    │   ├── java/Dockerfile      # Base Java image (extends node)
    │   └── dotnet/Dockerfile    # Base .NET image (extends node)
    └── scripts/
        ├── build-common.sh      # Shared build logic
        ├── clean-common.sh      # Shared clean logic
        ├── update-common.sh     # Shared update logic
        ├── repo-root.sh         # Resolves $REPO_ROOT
        └── run-common.sh        # Shared Docker run logic
```

## 🐛 Debugging

To get a shell inside a container instead of running the tool, override the entrypoint:

```bash
docker run --rm -it --entrypoint="" agentic-claude bash
```

Replace `agentic-claude` with the image you want to inspect (`agentic-copilot`, `agentic-opencode`, etc.). From there you can inspect the filesystem, check environment variables, or run the tool manually to see raw output.

## 🔒 Security

Containers run with the following constraints:

- Read-only filesystem
- All capabilities dropped
- No privilege escalation
- Runs as the host user to avoid permission issues on mounted files
- `/tmp` limited to 1GB
