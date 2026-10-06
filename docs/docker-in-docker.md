# Docker-in-Docker

> [!WARNING]
> **Beta** - Docker-in-Docker is under active development. Config keys and behavior may change between releases, and it hasn't had the same testing as the core container hardening. Issue reports are welcome.

Tools never get the host's Docker socket, because that would give the agent root on your machine. Instead, `--dind` starts a separate **rootless** Docker daemon in a per-run sidecar. The tool can then build images, run containers, use `docker compose`, or test devcontainers.

## Usage

```bash
agentic build claude --base docker   # once: add the Docker CLI layer
agentic claude --dind
```

Or enable it per project with [`[run.dind]`](config.md#rundind):

```toml
[run.dind]
enabled = true
memory = "8g" # sidecar only
```

The Docker host must allow unprivileged user namespaces (the default on Docker Desktop and most Linux distributions). If the sidecar fails to start, the error includes its logs. `agentic config` shows the resolved `[run.dind]` settings. See [Devcontainers](recipes.md#devcontainers) for an example.

## How it stays isolated

- **Tool container**: it keeps all of its [restrictions](security-model.md#layers) and only gains a client cert and the `DOCKER_*` env vars. `DOCKER_HOST`, `DOCKER_TLS_VERIFY`, `DOCKER_CERT_PATH`, `DOCKER_CONFIG` and `DOCKER_CONTEXT` are managed by agentic and can't be overridden.
- **Rootless daemon**: it runs as your user in a user namespace, never `--privileged`. Running agentic as root with `--dind` is refused.
- **Id mapping**: files that inner containers write to `/workspace` as root are owned by you. All other inner ids map to a range from 2,000,000,000 that no host account owns.
- **Sidecar privileges**: only the sidecar gets namespace-scoped capabilities, unconfined AppArmor and `/dev/net/tun`, which it needs to create the user namespace.
- **Seccomp**: Docker's default profile, minus kernel interfaces nested containers don't need (`bpf`, `perf_event_open`, `syslog`, ...). Keyring syscalls stay blocked.
- **Image**: `agentic-dind` is built locally from `docker:<version>-dind-rootless`, with all setuid/setgid bits removed. Only the id-map helpers get `CAP_SETUID`/`CAP_SETGID`. It is rebuilt with a fresh pull at least weekly.
- **Mutual TLS**: per-run throwaway certs, and the CA key never touches disk. Inner containers can't drive the daemon unless they bind-mount its socket or certs.
- **Filesystem**: the daemon sees only `/workspace` (`read_only_mounts` under it stay read-only), not the tool home, secrets or anything else.
- **Cleanup**: images, containers, volumes, certs and Docker config (including `docker login` credentials) are removed when the run ends. After a crash, the next `agentic` run removes them once they are a few minutes old, and `agentic clean` removes them right away.
- **Ports**: published ports are reachable from the tool at `agentic-docker:<port>`, never from the host.
- **With `--proxy`**: the sidecar shares the tool's internal network, so pulls, builds and containers go through the allowlist. The Docker CLI config passes the proxy to containers and builds. Docker Hub is allowed automatically. Add other registries (e.g. `ghcr.io`) to `allowed_hosts`.

## Trade-offs

- Inner containers get namespace-scoped `CAP_SYS_ADMIN`/`CAP_NET_ADMIN`, which exposes kernel interfaces (nf_tables, mounts, nested user namespaces) that a plain container can't reach. Keep the host kernel patched.
- The proxy filters by host only, so the agent can also push to an allowed registry.
- The sidecar's limits default to the tool's, so a run can use up to twice them. Images and volumes use the Docker host's disk with no size limit.
- An inner container can read the sidecar's own files, including its TLS key.
- Proxy-injected credentials don't work in inner containers (see [Limitations](egress-proxy.md#limitations)).

See [Security model](security-model.md) for how this fits with the other layers.
