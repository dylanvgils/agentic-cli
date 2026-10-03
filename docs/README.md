# Docs

1. [Overview](01-overview.md) - what agentic is, the security model, and motivation
2. [Configuration](02-config.md) - `.agenticrc.toml` format, merge semantics, and precedence rules
3. [Volume mounts](03-volume-mounts.md) - per-tool mount breakdown and why each mount exists
4. [Security model](04-security-model.md) - each isolation layer, what it stops, and the risk that's left
5. [Docker-in-Docker](05-docker-in-docker.md) - the rootless Docker sidecar: usage, isolation, trade-offs, devcontainers
6. [Dockerfile DSL](06-dockerfile-dsl.md) - the typed Go DSL used to generate Dockerfiles at build time
7. [Development](07-development.md) - build commands, Go conventions, adding tools and base runtimes
