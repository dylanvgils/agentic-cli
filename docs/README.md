# Docs

1. [Overview](overview.md) - what agentic is, the security model, and motivation
2. [Comparison](comparison.md) - agentic vs Docker Sandboxes (`sbx`) at a glance
3. [Installation](installation.md) - install, uninstall, upgrade, and building from source
4. [Usage](usage.md) - commands, shell setup, env vars, volumes, data directory, marketplaces
5. [Images](images.md) - base images, runtime layers, versions and updates, apt packages, registry proxy, install checksums
6. [Configuration](config.md) - `agentic.json` and `.agenticrc.toml` reference, merge and precedence rules
7. [Environment instructions](environment-instructions.md) - the block each run writes into the tool's instructions file
8. [Volume mounts](volume-mounts.md) - per-tool mount breakdown, why each mount exists, and mount safety checks
9. [Security model](security-model.md) - each isolation layer, what it stops, and the risk that's left
10. [Egress proxy](egress-proxy.md) - the allowlist proxy, its log, and private address blocking
11. [Credential injection](credential-injection.md) - API keys swapped in by the proxy, so the agent only sees a placeholder
12. [Docker-in-Docker](docker-in-docker.md) - the rootless Docker sidecar: usage, isolation, trade-offs
13. [Microsandbox runtime](microsandbox.md) - running a tool in a microVM instead of a container: requirements, differences, Docker in the VM
14. [Recipes](recipes.md) - Java build tools, Maven through the proxy, per-project images, Copilot without telemetry
15. [Dockerfile DSL](dockerfile-dsl.md) - the typed Go DSL used to generate Dockerfiles at build time
16. [Development](development.md) - build commands, repo layout, sidecar images, releasing
