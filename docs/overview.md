# Overview

Agentic CLI runs agentic coding tools (Claude Code, GitHub Copilot, OpenCode) in isolated Docker containers. A tool sees only your workspace and its own config directory. It runs without root, on a read-only filesystem, and leaves nothing behind when it exits.

## Security

Containers run read-only as your own user, with all capabilities dropped, no privilege escalation, resource limits, and an isolated network. A tool that needs to write somewhere gets a targeted mount, and nothing else is writable. Opt-in layers add an [egress proxy](egress-proxy.md) and a [Docker-in-Docker sidecar](docker-in-docker.md).

Each run also tells the model about these limits through [environment instructions](config.md#runinstructions).

See [Security model](security-model.md) for what each layer stops, and [Volume mounts](volume-mounts.md) for what's mounted.

## Motivation

Coding agents have guard rails, but they still run with all of your user's permissions. Agentic runs them in a locked-down container instead, so they can only touch what you hand them. It also makes daily use easy: one command to build or update a tool, and config that's picked up per project automatically.

[Docker Sandboxes](comparison.md) offers a VM boundary instead. Agentic is also a side project for learning to build AI-assisted tooling.
