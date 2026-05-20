# Agentic CLI

CLI for running agentic coding tools in isolated Docker containers.

## Contents

- [Overview](#-overview)
- [Requirements](#-requirements)
- [Installation](#-installation)
- [Usage](#-usage)
  - [Commands](#commands)
  - [Tools](#tools)
  - [Examples](#examples)
- [Shell completion](#-shell-completion)
- [Shell aliases](#-shell-aliases)
- [Base images](#-base-images)
- [Secrets](#-secrets)
- [Named Docker volumes](#-named-docker-volumes)
  - [Managing volumes](#managing-volumes)
- [Java build tools](#-java-build-tools)
- [Configuration](#-configuration)
  - [Per-project configuration](#per-project-configuration)
  - [Mount variable substitution](#mount-variable-substitution)
  - [Example `.zshrc`](#example-zshrc)
- [Tool home directory](#-tool-home-directory)
- [Development](docs/development.md)
- [Security](#-security)

## 📖 Overview

Each tool runs in an isolated, read-only container with only the minimal mounts it needs - your workspace and its own config directory. No root, no extra capabilities, no leftovers when done.

→ [Full overview and motivation](docs/overview.md)

## 📋 Requirements

- Docker
- Git

## 🚀 Installation

Clone the repo, then build and install using Docker (no Go required):

```bash
git clone https://github.com/dylanvgils/agentic-cli.git
cd agentic-cli
./install.sh        # Linux / macOS
.\install.ps1       # Windows (PowerShell)
```

On Linux/macOS, the binary is installed to `~/.local/bin`. If that directory isn't in your PATH, add it to your shell config:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

On Windows, the installer adds the install directory to your user PATH automatically. Restart your terminal after installation for the change to take effect.

To uninstall and remove all agentic data:

```bash
./install.sh --remove
.\install.ps1 -Remove
```

If you already have Go installed, you can build and install natively instead:

```bash
make install        # builds and installs to ~/.local/bin/agentic
make uninstall      # removes the binary
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

| Command                                                                                                                         | Description                                                                                 |
| ------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| `build [tool] [--base <e1[,e2,...]>] [--no-cache] [--node <version>] [--java <version>] [--dotnet <version>] [--go <version>]`  | Build tool image(s). Builds all tools if unspecified                                        |
| `update [tool] [--base <e1[,e2,...]>] [--no-cache] [--node <version>] [--java <version>] [--dotnet <version>] [--go <version>]` | Update tool image(s) to latest version. Skips unbuilt tools when unspecified                |
| `clean [tool]`                                                                                                                  | Remove tool image(s). Cleans all tools + base if unspecified                                |
| `inspect [tool]`                                                                                                                | Show image info (version, base layers, build date, size). Inspects all tools if unspecified |
| `config [--home <dir>]`                                                                                                         | Show the merged configuration from agentic.json and all .agenticrc files                    |
| `volumes <create\|list\|ls\|remove\|rm> [name]`                                                                                 | Manage named Docker volumes created by agentic                                              |
| `completion <bash\|zsh\|fish\|powershell>`                                                                                      | Generate shell completion script for the specified shell                                    |
| `aliases`                                                                                                                       | Print shell alias definitions for installed tools                                           |
| `help [command]`                                                                                                                | Show help for a command (`run` for tool run options). Shows overview if unspecified         |
| `run [flags] <tool> [args...]`                                                                                                  | Run a tool in an isolated Docker container                                                  |
| `run <tool> -- <cmd> [args]`                                                                                                    | Override the entrypoint and run a shell command directly                                    |

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

# Build with multiple extra runtimes (comma-separated, layered left to right)
agentic build claude --base java,dotnet

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
agentic run claude
agentic run copilot
agentic run opencode

# Run a shell command instead of the tool entrypoint
agentic run claude -- bash
agentic run claude -- ls /workspace

# Mount named Docker volumes (auto-created on first use)
agentic run -v 'maven:$CONTAINER_HOME/.m2' claude
agentic run -v 'maven:$CONTAINER_HOME/.m2' -v 'gradle:$CONTAINER_HOME/.gradle' claude

# Mount bind-mount volumes (host paths)
agentic run -v '~/.m2:$CONTAINER_HOME/.m2' claude
agentic run -v '~/.m2:$CONTAINER_HOME/.m2' -v '~/.gradle:$CONTAINER_HOME/.gradle' claude

# Mount a secret file read-only at /run/secrets/<name>
agentic run -s 'copilot_token:~/.secrets/copilot_token' copilot

# Override the tool home directory
agentic run --home /opt/agentic claude

# Print completion script
agentic completion zsh
agentic completion bash
```

## 🔁 Shell completion

Tab completion is available for bash and zsh. Add one of the following to your shell config to activate it:

```bash
# zsh - add to ~/.zshrc
source <(agentic completion zsh)

# bash - add to ~/.bashrc
source <(agentic completion bash)
```

Completions cover all commands (`build`, `update`, `clean`, `inspect`, `completion`, `aliases`, `help`), tool names, and command-specific flags (`--base`, `--no-cache`, `--help`). Tool names are discovered dynamically at completion time, so new tools are picked up automatically without regenerating the script.

## 🔗 Shell aliases

Shell aliases let you run tools directly (e.g., `copilot` instead of `agentic run copilot`). Add to your shell config to activate them:

```bash
source <(agentic aliases)
```

Only tools with a built image produce an alias, so sourcing the output never creates broken aliases for uninstalled tools.

## 🧱 Base images

Node is always the root layer. The `--base` flag adds extra runtimes on top of it:

```
node (base stage)
  ├── java   (java stage)   ← added with --base java
  ├── dotnet (dotnet stage) ← added with --base dotnet
  └── go     (go stage)     ← added with --base go
        └── tool (tool stage)
```

All stages are composed into a single multi-stage Dockerfile at build time and built in one `docker build` call. No intermediate images are produced.

| Flag                                 | Result                       |
| ------------------------------------ | ---------------------------- |
| _(none)_                             | node only (v24)              |
| `--base java`                        | node v24 + Java 21           |
| `--base dotnet`                      | node v24 + .NET 10           |
| `--base go`                          | node v24 + Go 1.26           |
| `--base java,dotnet`                 | node v24 + Java 21 + .NET 10 |
| `--node 22`                          | node v22 only                |
| `--base java --java 17`              | node v24 + Java 17           |
| `--base dotnet --dotnet 9`           | node v24 + .NET 9            |
| `--base go --go 1.23`                | node v24 + Go 1.23           |
| `--node 22 --base java --java 17`    | node v22 + Java 17           |
| `--node 22 --base dotnet --dotnet 9` | node v22 + .NET 9            |

Both tools default to node only. Use `--base` to add extra runtimes at build time.

Version defaults are `NODE_VERSION=24`, `JAVA_VERSION=21`, `DOTNET_VERSION=10`, `GO_VERSION=1.26.3`. Override them per-build with `--node`/`--java`/`--dotnet`/`--go`, or set `AGENTIC_NODE_VERSION`/`AGENTIC_JAVA_VERSION`/`AGENTIC_DOTNET_VERSION`/`AGENTIC_GO_VERSION` in your shell config for persistent defaults.

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

## 🔑 Secrets

Use `--secret` / `-s` to mount a secret file into the container at `/run/secrets/<name>`, read-only:

```bash
agentic run -s 'copilot_token:~/.secrets/copilot_token' copilot
```

For persistent global config, set `AGENTIC_SECRETS` in your shell:

```bash
export AGENTIC_SECRETS='copilot_token:~/.secrets/copilot_token'
```

For per-project control, use a [`.agenticrc` project config file](#per-project-configuration):

```sh
# .agenticrc
secrets=copilot_token:~/.secrets/copilot_token
```

Secrets use the format `name:/path/to/file`. The `~`, `$HOME`, and `${HOME}` prefixes are expanded to your home directory. The file is mounted read-only at `/run/secrets/<name>` inside the container.

## 📦 Named Docker volumes

The `-v` flag and `AGENTIC_EXTRA_MOUNTS` support both bind mounts (host paths) and named Docker volumes. Named volumes are created automatically on first use and persist across container runs — no host path required.

For a per-tool breakdown of what's mounted automatically and why, see [docs/volume-mounts.md](docs/volume-mounts.md).

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
extra_mounts=maven:$CONTAINER_HOME/.m2
extra_mounts=gradle:$CONTAINER_HOME/.gradle
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
extra_mounts=maven:$CONTAINER_HOME/.m2
extra_mounts=gradle:$CONTAINER_HOME/.gradle
```

## ⚙️ Configuration

All configuration is done through environment variables, which can be set in your shell config (`.zshrc`, `.bashrc`, etc.).

| Variable                 | Description                                                                                                                                           | Default                       |
| ------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------- |
| `AGENTIC_HOME`           | Base directory for tool config and secrets                                                                                                            | `$HOME/.agentic`              |
| `AGENTIC_EXTRA_MOUNTS`   | Comma-separated extra mounts. Bind mount: `host/path:container/path`. Named volume: `name:container/path` (auto-created). Supports `$CONTAINER_HOME`. | -                             |
| `AGENTIC_SECRETS`        | Comma-separated secrets to mount read-only at `/run/secrets/<name>`. Format: `name:/path/to/file`.                                                    | -                             |
| `AGENTIC_PIDS_LIMIT`     | Default container PID limit                                                                                                                           | `1024`                        |
| `AGENTIC_CPUS`           | Default container CPU limit                                                                                                                           | `4`                           |
| `AGENTIC_MEMORY`         | Default container memory limit                                                                                                                        | `4g`                          |
| `AGENTIC_NODE_VERSION`   | Node.js version used when building the base node image                                                                                                | `24` (Dockerfile default)     |
| `AGENTIC_JAVA_VERSION`   | Java (Temurin JDK) version used when building the java layer                                                                                          | `21` (Dockerfile default)     |
| `AGENTIC_DOTNET_VERSION` | .NET version used when building the dotnet layer                                                                                                      | `10` (Dockerfile default)     |
| `AGENTIC_GO_VERSION`     | Go version used when building the go layer                                                                                                            | `1.26.2` (Dockerfile default) |

### Per-project configuration

Place a `.agenticrc` file anywhere in your directory tree to apply project-specific configuration. `agentic` walks up from `$PWD` collecting all `.agenticrc` files it finds and merges them. Add `root=true` to a file to stop the walk there.

**Merge rules:** list keys (`extra_mounts`, `secrets`) accumulate from all levels, outermost first. Scalar keys (`cpus`, `memory`, `pids_limit`) use the innermost (child) value.

| Key            | Description                                                                                                                                                      | Default | Env var override       |
| -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------- | ---------------------- |
| `root`         | Stop walking up the directory tree at this file (`true`/`false`)                                                                                                 | `false` | -                      |
| `extra_mounts` | Extra mounts. Bind mount: `~/path:container/path`. Named volume: `name:container/path` (auto-created). Supports `~`, `$HOME`, and `$CONTAINER_HOME`. Repeatable. | -       | `AGENTIC_EXTRA_MOUNTS` |
| `secrets`      | Secrets to mount read-only at `/run/secrets/<name>`. Format: `name:/path/to/file`. Supports `~` and `$HOME`. Repeatable.                                         | -       | `AGENTIC_SECRETS`      |
| `pids_limit`   | Container PID limit                                                                                                                                              | `1024`  | `AGENTIC_PIDS_LIMIT`   |
| `cpus`         | Container CPU limit                                                                                                                                              | `4`     | `AGENTIC_CPUS`         |
| `memory`       | Container memory limit                                                                                                                                           | `4g`    | `AGENTIC_MEMORY`       |

`.agenticrc` values override env var defaults but are superseded by CLI flags. `extra_mounts` and `secrets` are appended to rather than replacing `AGENTIC_EXTRA_MOUNTS` / `AGENTIC_SECRETS`. You can commit `.agenticrc` to the repo so the whole team picks up the right settings automatically.

Repeatable keys let you list one entry per line; comma-separated values on a single line also work — your choice:

```sh
# .agenticrc
root=true

extra_mounts=maven:$CONTAINER_HOME/.m2
extra_mounts=gradle:$CONTAINER_HOME/.gradle

secrets=copilot_token:~/.secrets/copilot_token

pids_limit=2048
cpus=8
memory=8g
```

**Multi-level example** — shared secrets in a parent directory, project mounts in the project:

```sh
# ~/projects/.agenticrc  (applies to all projects under ~/projects)
root=true
secrets=gh-token:~/.secrets/gh_token

# ~/projects/my-project/.agenticrc
extra_mounts=maven:$CONTAINER_HOME/.m2
cpus=8
```

### Mount variable substitution

Several placeholders are substituted in mount strings at runtime. Use them so you don't have to hardcode paths that vary per machine or per tool:

| Placeholder         | Side of `:`       | Expands to                                     |
| ------------------- | ----------------- | ---------------------------------------------- |
| `~`                 | host (left)       | Your home directory                            |
| `$HOME`             | host (left)       | Same as above                                  |
| `${HOME}`           | host (left)       | Same as above                                  |
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
# export AGENTIC_HOME="${HOME}/.agentic"   # default; uncomment to override
# export AGENTIC_NODE_VERSION=22   # uncomment to pin Node.js version
# export AGENTIC_JAVA_VERSION=17   # uncomment to pin Java version
# export AGENTIC_DOTNET_VERSION=9  # uncomment to pin .NET version
# export AGENTIC_GO_VERSION=1.23   # uncomment to pin Go version

# Mount Maven and Gradle caches for Java projects (named volumes)
# export AGENTIC_EXTRA_MOUNTS='maven:$CONTAINER_HOME/.m2,gradle:$CONTAINER_HOME/.gradle'
```

## 🏠 Tool home directory

Each tool stores its configuration under `$AGENTIC_HOME`:

| Tool       | Config path                                                   |
| ---------- | ------------------------------------------------------------- |
| `claude`   | `$AGENTIC_HOME/claude/`, `$AGENTIC_HOME/claude/.claude.json`  |
| `copilot`  | `$AGENTIC_HOME/copilot/`                                      |
| `opencode` | `$AGENTIC_HOME/opencode/` (data, share, state, cache, config) |

## 🛠️ Development

See [docs/development.md](docs/development.md) for build commands, repo structure, adding tools, adding base runtimes, and debugging.

## 🔒 Security

Containers run with the following constraints:

- Read-only filesystem
- All capabilities dropped
- No privilege escalation
- Runs as the host user to avoid permission issues on mounted files
- `/tmp` limited to 1GB
