# Docs

## Getting started

- [Overview](overview.md) - what agentic is, the security model, and motivation
- [Installation](installation.md) - install, uninstall, upgrade, and building from source
- [Usage](usage.md) - commands, shell setup, env vars, volumes, data directory, marketplaces
- [Comparison](comparison.md) - agentic vs Docker Sandboxes (`sbx`) at a glance

## Configure

- [Configuration](config.md) - `agentic.json` and `.agenticrc.toml` reference, merge and precedence rules
- [Images](images.md) - base images, runtime layers, versions and updates, apt packages, registry proxy, install checksums
- [Recipes](recipes.md) - Java build tools, Maven through the proxy, per-project images, Copilot without telemetry

## Isolation

- [Security model](security-model.md) - each isolation layer, what it stops, and the risk that's left
- [Egress proxy](egress-proxy.md) - the allowlist proxy, its log, and private address blocking
- [Credential injection](credential-injection.md) - API keys swapped in by the proxy, so the agent only sees a placeholder
- [Docker-in-Docker](docker-in-docker.md) - the rootless Docker sidecar: usage, isolation, trade-offs
- [Microsandbox runtime](microsandbox.md) - running a tool in a microVM instead of a container: requirements, differences, Docker in the VM

## Reference

- [Volume mounts](volume-mounts.md) - per-tool mount breakdown, why each mount exists, and mount safety checks
- [Environment instructions](environment-instructions.md) - the block each run writes into the tool's instructions file

## Contributing

- [Development](development.md) - build commands, repo layout, sidecar images, releasing
- [Dockerfile DSL](dockerfile-dsl.md) - the typed Go DSL used to generate Dockerfiles at build time
