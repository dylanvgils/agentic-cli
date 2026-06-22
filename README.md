# Agentic CLI

Runs agentic coding tools in isolated, read-only Docker containers - each with only the minimal mounts it needs: your workspace and its own config directory. No root, no extra capabilities, no leftovers when done.

- **Multiple tools** - Claude Code, GitHub Copilot CLI, OpenCode, run the same way
- **Isolated by default** - read-only filesystem, no root, dropped capabilities, isolated network
- **Per-project config** - `.agenticrc.toml` files merge up the directory tree; namespaces keep separate image sets per project
- **Pluggable runtimes** - add Node.js, Java, .NET, or Go on top of the base image, with version pinning
- **Persistent state** - named volumes and read-only secret mounts survive across container runs
- **Egress allowlist proxy** - optionally restrict and log a tool's outbound network access

→ [Full overview and motivation](docs/01-overview.md)

## Contents

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
  - [Example `.zshrc`](#example-zshrc)
- [Tool home directory](#-tool-home-directory)
- [Development](docs/05-development.md)
- [Security](#-security)

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

To build from source instead of downloading a pre-built binary:

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

| Command                                                                                                                                                                                                             | Description                                                                                                                                                                          |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `build [tool] [--namespace <name>] [--base <extra>]... [--apt <pkg>]... [--no-cache] [--registry <host>] [--debian <version>] [--node <version>] [--java <version>] [--dotnet <version>] [--go <version>]`          | Build tool image(s). Builds all tools if unspecified                                                                                                                                 |
| `update [tool] [--namespace <name>] [--all] [--base <extra>]... [--apt <pkg>]... [--no-cache] [--registry <host>] [--debian <version>] [--node <version>] [--java <version>] [--dotnet <version>] [--go <version>]` | Update tool image(s) to latest version. `--all` updates every agentic image across all namespaces                                                                                    |
| `clean [tool] [--namespace <name>] [--all]`                                                                                                                                                                         | Remove tool image(s). `--all` removes across all namespaces. No-arg form also removes base images, the proxy image, leftover proxy resources, and the `agentic-net` network          |
| `proxy build [--no-cache] [--registry <host>] [--dry-run]`                                                                                                                                                          | Build the proxy image (`agentic-proxy`). Builds normally happen automatically the first time you run with `--proxy`; this is for forcing one explicitly                              |
| `proxy update [--registry <host>] [--dry-run]`                                                                                                                                                                      | Force a fresh proxy image build (always `--no-cache`), to pick up a proxy source or base-image change a cached image would otherwise mask                                            |
| `proxy clean [--logs]`                                                                                                                                                                                              | Remove the proxy image. The proxy image is global, not namespaced - there's only ever one. `--logs` also wipes all proxy access logs under `$AGENTIC_HOME/proxy/`, regardless of age |
| `inspect [tool] [--namespace <name>] [--all]`                                                                                                                                                                       | No arg: table of images in the active namespace; `--all` shows all namespaces. Tool arg: full detail for active namespace; `--all` shows all namespaces                              |
| `namespaces list` / `namespaces ls`                                                                                                                                                                                 | List all known namespaces                                                                                                                                                            |
| `namespaces prune [-n namespace]`                                                                                                                                                                                   | Remove all images in the active (or specified) namespace                                                                                                                             |
| `config [--home <dir>]`                                                                                                                                                                                             | Show the merged configuration from agentic.json and all .agenticrc.toml files                                                                                                        |
| `volumes <create\|list\|ls\|remove\|rm> [name]`                                                                                                                                                                     | Manage named Docker volumes created by agentic                                                                                                                                       |
| `upgrade [--force] [--version <tag>]`                                                                                                                                                                               | Upgrade the agentic binary to the latest release. `--force` reinstalls even if already up to date; `--version` installs a specific release tag                                       |
| `version`                                                                                                                                                                                                           | Show version information (version, commit, built by, built date)                                                                                                                     |
| `completion <bash\|zsh\|fish\|powershell>`                                                                                                                                                                          | Generate shell completion script for the specified shell                                                                                                                             |
| `aliases`                                                                                                                                                                                                           | Print shell alias definitions for installed tools                                                                                                                                    |
| `help [command]`                                                                                                                                                                                                    | Show help for a command (`run` for tool run options). Shows overview if unspecified                                                                                                  |
| `run [flags] <tool> [args...]`                                                                                                                                                                                      | Run a tool in an isolated Docker container. `--proxy` / `--no-proxy` toggle the egress allowlist proxy for the run                                                                   |
| `run <tool> -- <cmd> [args]`                                                                                                                                                                                        | Override the entrypoint and run a shell command directly                                                                                                                             |

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

# Update to latest version (checks upstream first; rebuilds only if newer, base layers stay cached)
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

`agentic update` reuses the version each layer was originally built with, so base/extra layers are regenerated identically (and stay cache-hits) even if the embedded defaults have since changed - pass the flag again to pin a different version instead. Pass `--no-cache` to also rebuild the base/extra layers from scratch, instead of only the tool stage.

> **Note:** During `agentic update`, the `bases` and `apt_packages` settings from `.agenticrc.toml` are ignored - the original build configuration is always reused. Only an explicit `--base` or `--apt` CLI flag (or the corresponding env var) overrides what the image was built with.

### Extra apt packages

Use `--apt` to install additional Debian packages into the base stage, verified against `apt-cache show` before the build starts (fail-fast):

```bash
agentic build claude --apt make
agentic build claude --apt make,gcc   # comma-separated or repeatable (--apt make --apt gcc)
```

`agentic update` automatically reuses the package list, so you don't need to re-specify it each time. For persisting packages via `AGENTIC_APT_PACKAGES` or `.agenticrc.toml`, and for registry proxy configuration (pulling base images through Harbor, Nexus, Artifactory, etc.), see [docs/02-config.md](docs/02-config.md).

Use `agentic inspect` to see base layers, apt packages, build timestamp, and installed tool version for any built image.

## 🔑 Secrets

Use `--secret` / `-s` to mount a secret file read-only into the container:

```bash
agentic run -s 'copilot_token:~/.secrets/copilot_token' copilot
```

Secrets use the format `name:/path/to/file[:/container/path]`. The `~`, `$HOME`, and `${HOME}` prefixes are expanded to your home directory. Without a container path the file is mounted at `/run/secrets/<name>`; with one it is mounted at the specified path (supports `$CONTAINER_HOME`):

```bash
# Mount Maven settings.xml at the path Maven expects
agentic run -s 'maven-settings:~/.m2/settings.xml:$CONTAINER_HOME/.m2/settings.xml' java-tool
```

For persisting secrets via `AGENTIC_SECRETS` or `.agenticrc.toml`, see [docs/02-config.md](docs/02-config.md).

## 📦 Named Docker volumes

The `-v` flag and `AGENTIC_EXTRA_MOUNTS` support both bind mounts (host paths) and named Docker volumes - named volumes are created automatically on first use and persist across container runs, no host path required. See [Examples](#examples) above for the mount syntax, [docs/02-config.md](docs/02-config.md) for `AGENTIC_EXTRA_MOUNTS` / `.agenticrc.toml` persistence, and [docs/volume-mounts.md](docs/03-volume-mounts.md) for a per-tool breakdown of what's mounted automatically and why.

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

Configuration comes from environment variables, `.agenticrc.toml` project files, and `agentic.json`, with CLI flags taking precedence over all of them. See [docs/02-config.md](docs/02-config.md) for the full environment variable reference, `.agenticrc.toml` format, merge rules, precedence, and mount variable expansion (`$TOOL_HOME`, `$CONTAINER_HOME`, etc.).

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

Containers run read-only with all capabilities dropped, no privilege escalation, and on an isolated Docker network - see [docs/01-overview.md](docs/01-overview.md#security-model) for the full list of constraints.

Optionally, an egress allowlist proxy can restrict a tool's outbound traffic to a configurable set of hosts and log every connection attempt - fail-closed, so anything not on the allowlist is blocked. Toggle it per run with `--proxy` / `--no-proxy`; see [docs/02-config.md](docs/02-config.md#keys) for the `[run.proxy]` config reference and setup details.
