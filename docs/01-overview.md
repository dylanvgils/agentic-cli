# Overview

Agentic CLI runs agentic coding tools (Claude Code, GitHub Copilot, OpenCode) inside isolated Docker containers. Each tool gets a read-only filesystem with only the mounts it needs - your workspace and its own config directory. Nothing else is accessible, no root, no leftover state when the container exits.

## Security model

Containers run with:

- Read-only filesystem
- All Linux capabilities dropped
- No privilege escalation (`no-new-privileges`)
- Host UID/GID mapping - the container process runs as your user, so file permissions on mounted directories work correctly
- `/tmp` limited to 1 GB

When a tool needs to write somewhere (config, cache, temp files), it gets a targeted mount - a named volume or bind mount for persistent state, or a tmpfs for ephemeral scratch space. Nothing gets write access unless explicitly granted.

See [volume-mounts.md](03-volume-mounts.md) for a per-tool breakdown of what's mounted and why.

## Motivation

Agentic coding tools are powerful - but that power comes at a cost. They do come with guard rails, but they still run with the same permissions as your user. You're trusting the tool not to access anything you didn't intend to give it - and that's a hard sell if you want to experiment without fully trusting the tool.

Docker does have a sandbox feature for this, but it's currently in early access and requires Docker Desktop. This project provides a solution that works with any Docker-compatible runtime - Rancher Desktop, Podman, or plain Docker. The container runs read-only with all capabilities dropped and no privilege escalation, so the tool can only touch what you explicitly hand it.

Beyond isolation, it also aims to make working with these tools practical day-to-day: a single command to build or update any tool, and a flexible configuration system that works globally or per-project so the right settings are always picked up automatically.

It's also a side project for learning how to build and work with AI-assisted tooling.
