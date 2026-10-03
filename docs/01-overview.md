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
- Optional egress allowlist proxy - restrict a tool to a configurable set of hosts and log every connection attempt (see [config.md](02-config.md#runproxy-section))
- Optional Docker-in-Docker - a per-run rootless Docker daemon in a separate sidecar, reached over mutual TLS; the tool container itself keeps every constraint above and never sees the host's Docker socket (see [config.md](02-config.md#rundind-section))

When a tool needs to write somewhere (config, cache, temp files), it gets a targeted mount - a named volume or bind mount for persistent state, or a tmpfs for ephemeral scratch space. Nothing gets write access unless explicitly granted.

See [volume-mounts.md](03-volume-mounts.md) for a per-tool breakdown of what's mounted and why.

These constraints (and what's installed) are also written into each tool's own global instructions file at run time, so the model itself knows what it can and can't do up front - see [Environment instructions](../README.md#-environment-instructions).

## Motivation

Agentic coding tools are powerful - but that power comes at a cost. They do come with guard rails, but they still run with the same permissions as your user. You're trusting the tool not to access anything you didn't intend to give it - and that's a hard sell if you want to experiment without fully trusting the tool.

Docker also offers [Docker Sandboxes](https://docs.docker.com/ai/sandboxes/) (`sbx`), which runs each agent in a microVM with its own kernel. That's a stronger boundary than containers, but it needs hardware virtualization and supports a narrower set of hosts (Apple Silicon macOS, Windows 11, and x86_64 Ubuntu with KVM at the time of writing). This project works with any Docker-compatible runtime - Rancher Desktop, Podman, or plain Docker, on any OS or architecture Docker runs on, including machines without nested virtualization. The container runs read-only with all capabilities dropped and no privilege escalation, so the tool can only touch what you explicitly hand it. If your platform supports it and you want a VM boundary, Docker Sandboxes is a good fit; if you need portability, deep per-project configuration, or a setup you can fully audit, this project is.

Beyond isolation, it also aims to make working with these tools practical day-to-day: a single command to build or update any tool, and a flexible configuration system that works globally or per-project so the right settings are always picked up automatically.

It's also a side project for learning how to build and work with AI-assisted tooling.
