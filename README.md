# Agentic CLI

Runs agentic coding tools in isolated, read-only Docker containers - each with only the minimal mounts it needs: your workspace and its own config directory. No root, no extra capabilities, no leftovers when done.

- **Multiple tools** - Claude Code, GitHub Copilot CLI, OpenCode, run the same way
- **Isolated by default** - read-only filesystem, no root, dropped capabilities, isolated network
- **Per-project config** - `.agenticrc.toml` files merge up the directory tree; namespaces keep separate image sets per project
- **Pluggable runtimes** - add Node.js, Java, .NET, or Go on top of the base image, with version pinning
- **Persistent state** - named volumes and read-only secret mounts survive across container runs
- **Egress allowlist proxy** - optionally restrict and log a tool's outbound network access, and inject API keys so the agent never sees them
- **Docker-in-Docker** - optionally give a tool its own rootless Docker daemon, never the host's socket

→ [Full overview and motivation](docs/overview.md)

## 📋 Requirements

- Docker
- Git

## 🚀 Installation

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/dylanvgils/agentic-cli/main/install.sh | bash

# Windows (PowerShell)
Invoke-RestMethod https://raw.githubusercontent.com/dylanvgils/agentic-cli/main/install.ps1 | Invoke-Expression
```

The installer fetches the latest release for your OS and architecture, verifies the checksum, and installs the binary. Update later with `agentic upgrade`. See [Installation](docs/installation.md) for uninstalling, building from source, and the Windows execution policy note.

## ⚡ Quick start

```bash
agentic build claude                  # build the image (once)
agentic build claude --base node,java # or with extra runtimes
agentic run claude                    # run it in the current git repo
```

To run tools directly (`claude` instead of `agentic run claude`), add aliases to your shell config:

```bash
source <(agentic aliases)   # bash/zsh
```

See [Usage](docs/usage.md) for fish/PowerShell, shell completion, and more examples.

## 🛠️ Commands

| Command                | Description                                            |
| ---------------------- | ------------------------------------------------------ |
| `run <tool> [args...]` | Run a tool in an isolated container                    |
| `build [tool]`         | Build tool image(s)                                    |
| `update [tool]`        | Update tool image(s) to the latest tool version        |
| `clean [tool]`         | Remove tool image(s), or everything agentic created    |
| `inspect [tool]`       | Show built images and what's in them                   |
| `config`               | Show the merged configuration                          |
| `status`               | Show the Docker backend and running agentic containers |
| `upgrade`              | Upgrade the agentic binary                             |

Tools: `claude` (Claude Code), `copilot` (GitHub Copilot CLI), `opencode` (OpenCode). See [Usage](docs/usage.md#commands) for every command, and `agentic help <command>` for flags.

## 🔒 Security

Containers run read-only with all capabilities dropped, no privilege escalation, your own uid/gid, and on an isolated Docker network. Opt-in layers add an egress allowlist proxy (`--proxy`), proxy-injected API keys the agent never sees (`[[run.proxy.credentials]]`), and a rootless Docker-in-Docker sidecar (`--dind`). See [Security model](docs/security-model.md) for what each layer stops and the risk that's left.

## 📚 Documentation

- [Overview](docs/overview.md) - what agentic is and why
- [Comparison](docs/comparison.md) - agentic vs Docker Sandboxes (`sbx`)
- [Installation](docs/installation.md) - install, uninstall, upgrade, build from source
- [Usage](docs/usage.md) - commands, examples, secrets, env vars, volumes, tool home
- [Images](docs/images.md) - base images, runtimes, versions, apt packages, custom installs
- [Configuration](docs/config.md) - `.agenticrc.toml` and `agentic.json` reference
- [Volume mounts](docs/volume-mounts.md) - what's mounted into each tool and why
- [Security model](docs/security-model.md) - isolation layers and leftover risk
- [Docker-in-Docker](docs/docker-in-docker.md) - the rootless Docker sidecar
- [Recipes](docs/recipes.md) - Java, Maven through the proxy, per-project images, devcontainers
- [Dockerfile DSL](docs/dockerfile-dsl.md) - how images are generated
- [Development](docs/development.md) - contributing, conventions, releasing
