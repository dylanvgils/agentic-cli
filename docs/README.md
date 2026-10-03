# Docs

1. [Overview](overview.md) - what agentic is, the security model, and motivation
2. [Installation](installation.md) - install, uninstall, upgrade, and building from source
3. [Usage](usage.md) - commands, examples, shell setup, secrets, env vars, volumes, and tool home
4. [Images](images.md) - base images, runtime layers, versions and updates, apt packages, custom installs
5. [Configuration](config.md) - `.agenticrc.toml` format, merge semantics, and precedence rules
6. [Volume mounts](volume-mounts.md) - per-tool mount breakdown and why each mount exists
7. [Security model](security-model.md) - each isolation layer, what it stops, and the risk that's left
8. [Docker-in-Docker](docker-in-docker.md) - the rootless Docker sidecar: usage, isolation, trade-offs
9. [Recipes](recipes.md) - Java build tools, Maven through the proxy, per-project images, devcontainers
10. [Dockerfile DSL](dockerfile-dsl.md) - the typed Go DSL used to generate Dockerfiles at build time
11. [Development](development.md) - build commands, Go conventions, adding tools and base runtimes
