# Overview

Agentic CLI runs agentic coding tools (Claude Code, GitHub Copilot, OpenCode) inside isolated Docker containers. Each tool gets a read-only filesystem with only the mounts it needs - your workspace and its own config directory. Nothing else is accessible, no root, no leftover state when the container exits.

## Security model

Containers run with:

- Read-only filesystem
- All Linux capabilities dropped
- No privilege escalation (`no-new-privileges`)
- Host UID/GID mapping - the container process runs as your user, so file permissions on mounted directories work correctly
- `/tmp` limited to 1 GB
- Isolated Docker network (`agentic-net`) - containers cannot reach other containers on the host, only the internet
- Optional egress allowlist proxy - restrict a tool to a configurable set of hosts and log every connection attempt (see [config.md](config.md#keys))
- Optional Docker-in-Docker - a per-run rootless Docker daemon in a separate sidecar, reached over mutual TLS; the tool container itself keeps every constraint above and never sees the host's Docker socket (see [docker-in-docker.md](docker-in-docker.md))

When a tool needs to write somewhere (config, cache, temp files), it gets a targeted mount - a named volume or bind mount for persistent state, or a tmpfs for ephemeral scratch space. Nothing gets write access unless explicitly granted.

See [volume-mounts.md](volume-mounts.md) for a per-tool breakdown of what's mounted and why, and [security-model.md](security-model.md) for what each layer stops and the risk that's left.

These constraints (and what's installed) are also written into each tool's own global instructions file at run time, so the model itself knows what it can and can't do up front - see [Environment instructions](usage.md#environment-instructions).

## Motivation

Agentic coding tools are powerful - but that power comes at a cost. They do come with guard rails, but they still run with the same permissions as your user. You're trusting the tool not to access anything you didn't intend to give it - and that's a hard sell if you want to experiment without fully trusting the tool. Agentic runs the tool in a locked-down container instead, so it can only touch what you explicitly hand it.

Beyond isolation, agentic also aims to make working with these tools practical day-to-day: a single command to build or update any tool, and a flexible configuration system that works globally or per-project so the right settings are always picked up automatically.

It's also a side project for learning how to build and work with AI-assisted tooling.

## Compared to Docker Sandboxes

Docker also offers [Docker Sandboxes](https://docs.docker.com/ai/sandboxes/) (`sbx`), which runs each agent in its own microVM - a stronger boundary than a container. The table compares agentic with local `sbx` sandboxes as of October 2026; check Docker's docs for current details.

| | agentic | Docker Sandboxes (local) |
| --- | --- | --- |
| **Boundary** | Container on the host's kernel | microVM with its own kernel |
| **Host requirements** | Any Docker-compatible runtime (Docker, Rancher Desktop, Podman) on any OS or architecture, no virtualization needed | macOS 14+ on Apple silicon, Windows 11 on x86_64, or Ubuntu 24.04+ on x86_64/arm64, with hardware virtualization |
| **Agent inside** | Non-root, read-only root filesystem, all capabilities dropped | `sudo` inside the VM; the VM is the boundary |
| **Network** | Open by default; opt-in allowlist proxy (`--proxy`) that logs every attempt | Deny by default; allowlist per machine or managed by your organization |
| **Credentials** | The agent can read its own token and any mounted secret | Injected by a host-side proxy; the agent never sees raw values |
| **Docker inside** | Opt-in rootless sidecar (`--dind`) | Private Docker Engine in every sandbox |
| **Config** | `.agenticrc.toml` per project, merged up the directory tree | `sbxenv.yaml` environment files (experimental) |
| **Account and cost** | None, free | Docker sign-in required; local sandboxes free |
| **Source** | Open source (MIT) | Published as binaries |

Docker also offers cloud sandboxes (`sbx --cloud`), which need no local virtualization but run on Docker's infrastructure with no access to your local files, billed per use.

If your platform supports it and you want a VM boundary or credentials the agent can't read, Docker Sandboxes is a good fit. If you need portability, deep per-project configuration, or a setup you can fully audit, agentic is.
