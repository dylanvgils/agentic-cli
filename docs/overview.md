# Overview

Agentic CLI runs agentic coding tools (Claude Code, GitHub Copilot, OpenCode) in isolated Docker containers. A tool sees only your workspace and its own config directory. It runs without root, on a read-only filesystem, and leaves nothing behind when it exits.

## Security

Containers run read-only as your own user, with all capabilities dropped, no privilege escalation, resource limits, and an isolated network. A tool that needs to write somewhere gets a targeted mount, and nothing else is writable. Opt-in layers add an [egress proxy](egress-proxy.md) and a [Docker-in-Docker sidecar](docker-in-docker.md).

Each run also tells the model about these limits through [environment instructions](config.md#runinstructions).

See [Security model](security-model.md) for what each layer stops, and [Volume mounts](volume-mounts.md) for what's mounted.

## Motivation

Agentic coding tools are powerful - but that power comes at a cost. They do come with guard rails, but they still run with the same permissions as your user. You're trusting the tool not to access anything you didn't intend to give it - and that's a hard sell if you want to experiment without fully trusting the tool. Agentic runs the tool in a locked-down container instead, so it can only touch what you explicitly hand it. Docker Sandboxes offers a VM boundary instead - see [Comparison](comparison.md) for the trade-offs.

Beyond isolation, agentic also aims to make working with these tools practical day-to-day: a single command to build or update any tool, and a flexible configuration system that works globally or per-project so the right settings are always picked up automatically.

It's also a side project for learning how to build and work with AI-assisted tooling.
