# Agentic CLI

CLI for running agentic coding tools in isolated Docker containers.

## Contents

- [Overview](#-overview)
- [Requirements](#-requirements)
- [Installation](#-installation)
  - [Updating](#updating)
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
  - [Mount variable expansion](#mount-variable-expansion)
  - [Example `.zshrc`](#example-zshrc)
- [Tool home directory](#-tool-home-directory)
- [Development](docs/05-development.md)
- [Security](#-security)

## 📖 Overview

Each tool runs in an isolated, read-only container with only the minimal mounts it needs - your workspace and its own config directory. No root, no extra capabilities, no leftovers when done.

→ [Full overview and motivation](docs/01-overview.md)

## 📋 Requirements

- Docker
- Git

## 🚀 Installation

Install directly:

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/dylanvgils/agentic-cli/main/install.sh | bash

# Windows (PowerShell)
Invoke-RestMethod https://raw.githubusercontent.com/dylanvgils/agentic-cli/main/install.ps1 | Invoke-Expression
```

Or clone the repo first and run the script from there:

```bash
git clone https://github.com/dylanvgils/agentic-cli.git
cd agentic-cli
./install.sh        # Linux / macOS
.\install.ps1       # Windows (PowerShell)
```

The installer fetches the latest release for your OS and architecture, verifies the checksum, and installs the binary.

> **Windows note:** If you get an error about running scripts being disabled, set the execution policy for your user:
>
> ```powershell
> Set-ExecutionPolicy -Scope CurrentUser -ExecutionPolicy RemoteSigned
> ```
>
> Alternatively, run it directly without changing the policy:
>
> ```powershell
> powershell -ExecutionPolicy Bypass -File .\install.ps1
> ```

On Linux/macOS, the binary is installed to `~/.local/bin`. If that directory isn't in your PATH, add it to your shell config:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

On Windows, the installer adds the install directory to your user PATH automatically. Restart your terminal after installation for the change to take effect.

To uninstall and remove all agentic data:

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/dylanvgils/agentic-cli/main/install.sh | bash -s -- --remove

# Windows (PowerShell)
& ([scriptblock]::Create((Invoke-RestMethod https://raw.githubusercontent.com/dylanvgils/agentic-cli/main/install.ps1))) -Remove
```

Or if you already have the repo cloned:

```bash
./install.sh --remove
.\install.ps1 -Remove
```

### Updating

Once installed, use the `upgrade` command to upgrade to the latest release:

```bash
agentic upgrade                    # update to latest
agentic upgrade --force            # reinstall even if already up to date
agentic upgrade --version v1.2.0   # install a specific release
```

The CLI also checks for updates automatically once per day and prompts you when a newer release is available.

### Building from source

To build from source instead of downloading a pre-built binary (requires Docker):

```bash
./install.sh --from-source    # Linux / macOS
.\install.ps1 -FromSource     # Windows (PowerShell)
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
agentic build claude --base node,java # Claude with Node.js and Java runtimes added
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

| Command                                                                                                                                                                                                             | Description                                                                                                                                             |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `build [tool] [--namespace <name>] [--base <extra>]... [--apt <pkg>]... [--no-cache] [--registry <host>] [--debian <version>] [--node <version>] [--java <version>] [--dotnet <version>] [--go <version>]`          | Build tool image(s). Builds all tools if unspecified                                                                                                    |
| `update [tool] [--namespace <name>] [--all] [--base <extra>]... [--apt <pkg>]... [--no-cache] [--registry <host>] [--debian <version>] [--node <version>] [--java <version>] [--dotnet <version>] [--go <version>]` | Update tool image(s) to latest version. `--all` updates every agentic image across all namespaces                                                       |
| `clean [tool] [--namespace <name>] [--all]`                                                                                                                                                                         | Remove tool image(s). `--all` removes across all namespaces. No-arg form also removes base images, the proxy image, leftover proxy resources, and the `agentic-net` network |
| `inspect [tool] [--namespace <name>] [--all]`                                                                                                                                                                       | No arg: table of images in the active namespace; `--all` shows all namespaces. Tool arg: full detail for active namespace; `--all` shows all namespaces |
| `namespaces list` / `namespaces ls`                                                                                                                                                                                 | List all known namespaces                                                                                                                               |
| `namespaces prune [-n namespace]`                                                                                                                                                                                   | Remove all images in the active (or specified) namespace                                                                                                |
| `config [--home <dir>]`                                                                                                                                                                                             | Show the merged configuration from agentic.json and all .agenticrc.toml files                                                                           |
| `volumes <create\|list\|ls\|remove\|rm> [name]`                                                                                                                                                                     | Manage named Docker volumes created by agentic                                                                                                          |
| `upgrade [--force] [--version <tag>]`                                                                                                                                                                               | Upgrade the agentic binary to the latest release. `--force` reinstalls even if already up to date; `--version` installs a specific release tag          |
| `version`                                                                                                                                                                                                           | Show version information (version, commit, built by, built date)                                                                                        |
| `completion <bash\|zsh\|fish\|powershell>`                                                                                                                                                                          | Generate shell completion script for the specified shell                                                                                                |
| `aliases`                                                                                                                                                                                                           | Print shell alias definitions for installed tools                                                                                                       |
| `help [command]`                                                                                                                                                                                                    | Show help for a command (`run` for tool run options). Shows overview if unspecified                                                                     |
| `run [flags] <tool> [args...]`                                                                                                                                                                                      | Run a tool in an isolated Docker container. `--proxy` / `--no-proxy` toggle the egress allowlist proxy for the run                                       |
| `run <tool> -- <cmd> [args]`                                                                                                                                                                                        | Override the entrypoint and run a shell command directly                                                                                                |

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
agentic build claude

# Build with extra runtimes on top of debian (comma-separated or repeatable)
agentic build claude --base node
agentic build claude --base node,java
agentic build claude --base node,java,dotnet

# Pin runtime versions
agentic build claude --base node,java --java 17
agentic build claude --base node --node 22

# Install extra apt packages (comma-separated or repeatable)
agentic build claude --apt make
agentic build claude --apt make,gcc

# Force a fully fresh build
agentic build claude --no-cache

# Update to latest version (only rebuilds the tool step, base layers stay cached)
agentic update
agentic update claude --base node,java
agentic update claude --no-cache   # also rebuilds base layers

# Clean / inspect images
agentic clean
agentic clean claude
agentic inspect                      # table of all agentic images in the active namespace
agentic inspect claude               # full detail for active namespace's claude image
agentic inspect claude --all         # detail for all namespaces' claude image

# Build a project-specific image set (images are named <namespace>-<tool>)
agentic build claude --namespace myproject --base node,java --apt make
agentic inspect                      # shows both agentic-claude and myproject-claude
AGENTIC_NAMESPACE=myproject agentic run claude
agentic update --all                 # update every agentic image across all namespaces

# Run a tool
agentic run claude

# Run a shell command instead of the tool entrypoint
agentic run claude -- bash

# Mount named Docker volumes (auto-created on first use)
agentic run -v 'maven:$CONTAINER_HOME/.m2' -v 'gradle:$CONTAINER_HOME/.gradle' claude

# Mount bind-mount volumes (host paths)
agentic run -v '~/.m2:$CONTAINER_HOME/.m2' claude

# Mount a secret file read-only at /run/secrets/<name>
agentic run -s 'copilot_token:~/.secrets/copilot_token' copilot

# Override the tool home directory
agentic run --home /opt/agentic claude

# Print completion script
agentic completion zsh
```

## 🔁 Shell completion

Tab completion is available for bash, zsh, fish, and PowerShell. Add one of the following to your shell config to activate it:

```bash
# zsh - add to ~/.zshrc
source <(agentic completion zsh)

# bash - add to ~/.bashrc
source <(agentic completion bash)

# fish - add to ~/.config/fish/config.fish
agentic completion fish | source

# PowerShell - add to your $PROFILE
agentic completion powershell | Out-String | Invoke-Expression
```

Tool names are discovered dynamically at completion time, so new tools are picked up automatically without regenerating the script.

## 🔗 Shell aliases

Shell aliases let you run tools directly (e.g., `copilot` instead of `agentic run copilot`). The shell is detected automatically. Add to your shell config to activate them:

```bash
# bash/zsh - add to ~/.bashrc or ~/.zshrc
source <(agentic aliases)

# fish - add to ~/.config/fish/config.fish
agentic aliases | source

# PowerShell - add to your $PROFILE
agentic aliases | Out-String | Invoke-Expression
```

Only tools with a built image produce an alias, so sourcing the output never creates broken aliases for uninstalled tools.

## 🧱 Base images

Debian is the root layer. The `--base` flag adds extra runtimes on top of it, including Node.js (installed via NVM):

```
debian (base stage)
  ├── dotnet (dotnet stage) ← added with --base dotnet
  ├── go     (go stage)     ← added with --base go
  ├── java   (java stage)   ← added with --base java
  └── node   (node stage)   ← added with --base node
        └── tool (tool stage)
```

All stages are composed into a single multi-stage Dockerfile at build time and built in one `docker build` call. No intermediate images are produced.

| Flag                                   | Result                         |
| -------------------------------------- | ------------------------------ |
| _(none)_                               | debian only                    |
| `--base node`                          | debian + Node.js               |
| `--base node,java`                     | debian + Node.js + Java        |
| `--base node,java,dotnet`              | debian + Node.js + Java + .NET |
| `--node 22`                            | debian + Node.js v22           |
| `--base node,java --java 17`           | debian + Node.js + Java 17     |
| `--node 22 --base node,java --java 17` | debian + Node.js v22 + Java 17 |

Use `--base` to add extra runtimes at build time. The same pinning pattern applies to every layer (`--debian`, `--node`, `--dotnet`, `--go`, etc.).

Version defaults are embedded in the binary at build time - run `agentic build --help` to see current defaults. Override per-build with the corresponding flag (`--debian`, `--node`, `--java`, `--dotnet`, `--go`), or set `AGENTIC_<LAYER>_VERSION` in your shell config for a persistent default (e.g. `AGENTIC_JAVA_VERSION=17`, `AGENTIC_NODE_VERSION=22`).

The resolved version for each layer is stored in the `agentic.version-args` image label and automatically recovered on `agentic update`, so the base/extra layers are regenerated identically (and stay cache-hits) even if the embedded defaults have since changed - pass the flag again to pin a different version instead. Pass `--no-cache` to `agentic update` to bypass this and rebuild the base/extra layers from scratch as well, instead of only the tool stage.

> **Note:** During `agentic update`, the `bases` and `apt_packages` settings from `.agenticrc.toml` are ignored - the image's own labels are always used to recover the original build configuration. Only an explicit `--base` or `--apt` CLI flag (or the corresponding env var) overrides what the image was built with.

### Registry proxy

If your environment requires pulling Docker Hub images through a registry proxy (e.g. Harbor, Nexus, Artifactory, AWS ECR pull-through cache), set the registry hostname and agentic will prefix all base image pulls with it:

```bash
# One-off override via flag
agentic build claude --registry myregistry.example.com

# Persistent: add to $AGENTIC_HOME/agentic.json (default: ~/.agentic/agentic.json)
# { "registry": "myregistry.example.com" }
```

Authentication is out of scope - configure it once with `docker login myregistry.example.com` before building.

The `--registry` flag takes precedence over the `agentic.json` value. Run `agentic config` to see the active registry setting.

### Extra apt packages

Use `--apt` to install additional Debian packages into the base stage. Packages are installed before any extra runtime layers, so they are available everywhere:

```bash
agentic build claude --apt make
agentic build claude --apt make,gcc   # comma-separated or repeatable (--apt make --apt gcc)
```

Packages are verified with `apt-cache show` before the build starts (fail-fast). The package list is stored in the `agentic.apt` image label and automatically recovered on `agentic update`, so you don't need to re-specify it each time.

You can also set packages persistently via environment variable or `.agenticrc.toml`:

```bash
# Environment variable (comma-separated)
export AGENTIC_APT_PACKAGES=make,gcc
```

```toml
# .agenticrc.toml (accumulates across nested RC files)
[build]
apt_packages = ["make", "gcc"]
```

All three sources accumulate: RC files (outermost first), then `AGENTIC_APT_PACKAGES`, then `--apt`.

Use `agentic inspect` to see base layers, apt packages, build timestamp, and installed tool version for any built image.

## 🔑 Secrets

Use `--secret` / `-s` to mount a secret file read-only into the container:

```bash
agentic run -s 'copilot_token:~/.secrets/copilot_token' copilot
```

For persistent global config, set `AGENTIC_SECRETS` in your shell:

```bash
export AGENTIC_SECRETS='copilot_token:~/.secrets/copilot_token'
```

For per-project control, use a [`.agenticrc.toml` project config file](#per-project-configuration):

```toml
# .agenticrc.toml
[run]
secrets = ["copilot_token:~/.secrets/copilot_token"]
```

Secrets use the format `name:/path/to/file[:/container/path]`. The `~`, `$HOME`, and `${HOME}` prefixes are expanded to your home directory. Without a container path the file is mounted at `/run/secrets/<name>`; with one it is mounted at the specified path (supports `$CONTAINER_HOME`):

```bash
# Mount Maven settings.xml at the path Maven expects
agentic run -s 'maven-settings:~/.m2/settings.xml:$CONTAINER_HOME/.m2/settings.xml' java-tool
```

## 📦 Named Docker volumes

The `-v` flag and `AGENTIC_EXTRA_MOUNTS` support both bind mounts (host paths) and named Docker volumes. Named volumes are created automatically on first use and persist across container runs - no host path required.

For a per-tool breakdown of what's mounted automatically and why, see [docs/volume-mounts.md](docs/03-volume-mounts.md).

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

For per-project control, use a [`.agenticrc.toml` project config file](#per-project-configuration):

```toml
# .agenticrc.toml
[run]
extra_mounts = [
  "maven:$CONTAINER_HOME/.m2",
  "gradle:$CONTAINER_HOME/.gradle",
]
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

Or add to `.agenticrc.toml` in the repo root so the whole team picks it up:

```toml
# .agenticrc.toml
[run]
extra_mounts = [
  "maven:$CONTAINER_HOME/.m2",
  "gradle:$CONTAINER_HOME/.gradle",
]
```

## ⚙️ Configuration

All configuration is done through environment variables, which can be set in your shell config (`.zshrc`, `.bashrc`, etc.).

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

### Per-project configuration

Place a `.agenticrc.toml` file anywhere in your directory tree to apply project-specific configuration. `agentic` walks up from `$PWD` collecting all `.agenticrc.toml` files it finds and merges them. Add `root = true` to a file to stop the walk there.

> **Migration note:** The old `.agenticrc` key=value format is no longer supported. If `agentic` finds a `.agenticrc` file it will print a warning. Rename it to `.agenticrc.toml` and convert the contents to TOML.

**Merge rules:** list keys (`bases`, `apt_packages`, `extra_mounts`, `secrets`) accumulate from all levels, outermost first. Scalar keys (`cpus`, `memory`, `pids_limit`) use the innermost (child) value. `namespace` and `versions` keys also use the innermost value - for `versions`, each layer name is resolved independently so a child can pin `java` without affecting `node` inherited from a parent. `.agenticrc.toml` takes precedence over env vars for all scalar keys.

`root` and `namespace` are top-level keys. Build-time settings go under `[build]`; runtime settings go under `[run]`.

**Top-level**

| Key         | Description                                             | Default   | Env var override    |
| ----------- | ------------------------------------------------------- | --------- | ------------------- |
| `root`      | Stop walking up the directory tree at this file         | `false`   | -                   |
| `namespace` | Image namespace; images are named `<namespace>-<tool>`. | `agentic` | `AGENTIC_NAMESPACE` |

**`[build]` section** - baked into the image at build time

| Key            | Description                                                                                          | Default | Env var override          |
| -------------- | ---------------------------------------------------------------------------------------------------- | ------- | ------------------------- |
| `bases`        | Extra runtime layers to add on top of node (e.g. `["java", "dotnet"]`). Accumulates with `--base`.   | -       | -                         |
| `apt_packages` | Extra apt packages to install at build time. Accumulates with `--apt` and env var.                   | -       | `AGENTIC_APT_PACKAGES`    |
| `versions`     | Per-layer version pins as a TOML table (e.g. `[build.versions]` with `java = "17"`). Innermost wins. | -       | `AGENTIC_<LAYER>_VERSION` |

**`[run]` section** - applied to each container at runtime

| Key            | Description                                                                                                                                                                                           | Default | Env var override       |
| -------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------- | ---------------------- |
| `extra_mounts` | Extra mounts. Bind mount: `~/path:container/path`. Named volume: `name:container/path` (auto-created). Supports `~`, `$HOME`, and `$CONTAINER_HOME`.                                                  | -       | `AGENTIC_EXTRA_MOUNTS` |
| `secrets`      | Secrets to mount read-only into the container. Format: `name:/path/to/file[:/container/path]`. Defaults to `/run/secrets/<name>`. Supports `~`, `$HOME`, and `$CONTAINER_HOME` (container path only). | -       | `AGENTIC_SECRETS`      |
| `pids_limit`   | Container PID limit (quoted string, e.g. `"1024"`)                                                                                                                                                    | `1024`  | `AGENTIC_PIDS_LIMIT`   |
| `cpus`         | Container CPU limit (quoted string, e.g. `"4"`)                                                                                                                                                       | `4`     | `AGENTIC_CPUS`         |
| `memory`       | Container memory limit (string, e.g. `"8g"`)                                                                                                                                                          | `4g`    | `AGENTIC_MEMORY`       |

**`[run.proxy]` section** - egress allowlist proxy (see [Egress proxy](#egress-proxy))

| Key             | Description                                                                                                                            | Default | Env var override |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------- | ------- | ---------------- |
| `enabled`       | Route the tool's egress through the allowlist proxy. Override per run with `--proxy` / `--no-proxy`.                                   | `false` | -                |
| `allowed_hosts` | Extra hosts to permit, merged on top of the tool's baseline. Exact match, or a leading-dot/`*.` entry to match a domain + subdomains.  | -       | -                |

You can commit `.agenticrc.toml` to the repo so the whole team picks up the right settings automatically.

```toml
# .agenticrc.toml
root = true

[build]
bases = ["java"]
apt_packages = ["make", "gcc"]

[build.versions]
java = "17"
node = "22"

[run]
extra_mounts = [
  "maven:$CONTAINER_HOME/.m2",
  "gradle:$CONTAINER_HOME/.gradle",
]
secrets = ["copilot_token:~/.secrets/copilot_token"]
pids_limit = "2048"
cpus = "8"
memory = "8g"
```

**Multi-level example** - shared secrets in a parent directory, project mounts in the project:

```toml
# ~/projects/.agenticrc.toml  (applies to all projects under ~/projects)
root = true

[build]
bases = ["java"]
apt_packages = ["make"]

[build.versions]
node = "22"

[run]
secrets = ["gh-token:~/.secrets/gh_token"]
```

```toml
# ~/projects/my-project/.agenticrc.toml
[build]
apt_packages = ["gcc"]

[build.versions]
java = "17"  # pins java; node = "22" is inherited from parent

[run]
extra_mounts = ["maven:$CONTAINER_HOME/.m2"]
cpus = "8"
```

### Mount variable expansion

Several placeholders are expanded in mount strings at runtime. Use them so you don't have to hardcode paths that vary per machine or per tool:

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
# export AGENTIC_NODE_VERSION=22   # pin a runtime version (see agentic build --help for all layers)

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

See [docs/development.md](docs/05-development.md) for build commands, repo structure, adding tools, adding base runtimes, and debugging.

## 🔒 Security

Containers run with the following constraints:

- Read-only filesystem
- All capabilities dropped
- No privilege escalation
- Runs as the host user to avoid permission issues on mounted files
- `/tmp` limited to 1GB
- Isolated Docker network (`agentic-net`) - containers cannot reach other containers on the host, only the internet
- Optional egress allowlist proxy - restrict a tool to a configurable set of hosts (see below)

### Egress proxy

By default a tool container can reach any host on the internet. Enable the egress proxy to restrict it to an allowlist and record every host it contacts.

When enabled, the tool no longer has direct internet access. Instead it runs on a per-run **internal** Docker network and reaches the outside world only through a dedicated proxy sidecar that enforces the allowlist. This is **fail-closed**: anything not routed through the proxy simply cannot connect, and any host not on the allowlist is blocked with a `403`. The proxy matches on hostname only (via HTTP `CONNECT`); it does not decrypt TLS.

Each tool ships a baseline allowlist of the hosts it needs (e.g. Claude Code allows `.anthropic.com`). Add your own with `allowed_hosts`:

```toml
# .agenticrc.toml
[run.proxy]
enabled = true
allowed_hosts = [
  "registry.npmjs.org",
  ".github.com",      # leading dot matches the domain and any subdomain
]
```

- Toggle per run with `--proxy` / `--no-proxy` (overrides the config value).
- Matching is exact, or a leading-dot / `*.` entry matches a domain and its subdomains. Ports 80 and 443 are allowed.
- Blocked hosts are summarized at the end of the run; add them to `allowed_hosts` to permit.
- Every connection attempt is logged as JSON lines under `$AGENTIC_HOME/proxy/`.

The proxy image is built automatically by `agentic build`, and on demand the first time you run with `--proxy` if it is missing. A released agentic installs its matching version from the published module; a dev build (version `dev`) compiles the proxy from the local source tree, so build it by running `agentic build` from within the agentic repository. `build` and `update` themselves are not proxied - they need broad network access for apt and package installs.
