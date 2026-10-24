# Agentic CLI

[![CI](https://github.com/dylanvgils/agentic-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/dylanvgils/agentic-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/dylanvgils/agentic-cli)](https://github.com/dylanvgils/agentic-cli/releases/latest)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=agentic-cli&metric=coverage)](https://sonarcloud.io/summary/new_code?id=agentic-cli)

Runs agentic coding tools in isolated, read-only Docker containers, or optionally in a microVM - each with only the minimal mounts it needs: your workspace and its own config directory. No root, no extra capabilities, no leftovers when done.

> [!NOTE]
> agentic is pre-1.0 and under active development, so flags, config keys and behavior may change between minor releases. See [Security model](docs/security-model.md#layers) for how well each isolation layer is tested.

- **Multiple tools** - Claude Code, GitHub Copilot CLI, OpenCode, run the same way
- **Isolated by default** - read-only filesystem, no root, dropped capabilities, isolated network
- **Per-project config** - `.agenticrc.toml` files merge up the directory tree; namespaces keep separate image sets per project
- **Language runtimes** - add Node.js, Java, .NET or Go on top of the base image, with version pinning
- **Persistent state** - named volumes survive across container runs
- **Egress allowlist proxy** - optionally restrict and log a tool's outbound network access
- **Credential injection** - API keys the agent never sees, only a placeholder
- **Docker-in-Docker** - optionally give a tool its own Docker daemon, never the host's socket
- **Microsandbox runtime** - optionally run a tool in a microVM with its own kernel instead of a container

→ [Full overview and motivation](docs/overview.md)

## 📋 Requirements

- Docker Engine 26+ (or Rancher Desktop)
- Git
- Optional: [msb](docs/microsandbox.md#requirements) 0.7.8+ for the microVM runtime

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
agentic run claude                    # run it in the current directory, then trust it and sign in
```

To run tools directly (`claude` instead of `agentic run claude`) and get `agt` as shorthand for `agentic`, add aliases to your shell config:

```bash
source <(agentic aliases)   # bash/zsh
```

See [First run](docs/usage.md#first-run) for the trust prompt and signing in, and [Usage](docs/usage.md) for fish/PowerShell, shell completion, and more examples.

## 🛠️ Commands

| Command                | Description                                            |
| ---------------------- | ------------------------------------------------------ |
| `run <tool> [args...]` | Run a tool in an isolated container                    |
| `build [tool]`         | Build tool image(s)                                    |
| `update [tool]`        | Update tool image(s) to the latest tool version        |
| `clean [tool]`         | Remove tool image(s), or everything agentic created    |
| `images`               | List built images                                      |
| `inspect <tool>`       | Show what's in a tool's image                          |
| `config`               | Show the merged configuration                          |
| `ps`                   | List running agentic containers and VMs                |
| `status`               | Show the Docker and microsandbox backends              |
| `upgrade`              | Upgrade the agentic binary                             |

Tools: `claude` (Claude Code), `copilot` (GitHub Copilot CLI), `opencode` (OpenCode). See [Usage](docs/usage.md#commands) for every command, and `agentic help <command>` for flags.

## 🔒 Security

Containers run read-only with all capabilities dropped, no privilege escalation, your own uid/gid, and on an isolated Docker network. The proxy, credential injection, Docker daemon and microVM are opt-in layers on top. See [Security model](docs/security-model.md) for what each layer stops.

What it doesn't protect against:

- **Credentials it can read**: the agent can read its own login token and any file you mount. Use [credential injection](docs/credential-injection.md) for API keys.
- **Workspace changes**: the agent can edit git hooks, `Makefile` and scripts that later run on your machine. Review diffs.
- **Network access**: without `--proxy` the tool reaches the whole internet, and with it, any allowed host. See [Egress proxy](docs/egress-proxy.md).

See [Leftover risk](docs/security-model.md#leftover-risk) for the full list.

## 📚 Documentation

- [Overview](docs/overview.md) - what agentic is and why
- [Comparison](docs/comparison.md) - agentic vs Docker Sandboxes (`sbx`)
- [Installation](docs/installation.md) - install, uninstall, upgrade, build from source
- [Usage](docs/usage.md) - commands, examples, env vars, volumes, marketplaces
- [Images](docs/images.md) - base images, runtimes, versions, apt packages, registry proxy
- [Configuration](docs/config.md) - `.agenticrc.toml` and `agentic.json` reference
- [Environment instructions](docs/environment-instructions.md) - what each run tells the model about its sandbox
- [Volume mounts](docs/volume-mounts.md) - what's mounted into each tool, and mount safety checks
- [Security model](docs/security-model.md) - isolation layers and leftover risk
- [Egress proxy](docs/egress-proxy.md) - the allowlist proxy and its access log
- [Credential injection](docs/credential-injection.md) - API keys the agent never sees
- [Docker-in-Docker](docs/docker-in-docker.md) - a Docker daemon for the tool, never the host's
- [Microsandbox runtime](docs/microsandbox.md) - running a tool in a microVM instead of a container
- [Recipes](docs/recipes.md) - Java, Maven through the proxy, per-project images, Copilot without telemetry
- [Development](docs/development.md) - contributing, conventions, releasing
