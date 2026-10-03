# Docker-in-Docker

Tools never get the host's Docker socket - that would hand the agent root on your machine. Instead, `--dind` (or `enabled = true` under `[run.dind]`) starts a separate, per-run **rootless** Docker daemon in a sidecar container, so the tool can build images, run containers, use `docker compose`, or test devcontainers.

## Usage

```bash
# The image needs the Docker CLI layer (once)
agentic build claude --base docker

# Run with its own Docker daemon
agentic claude --dind
```

Or enable it per project in `.agenticrc.toml` - see [`[run.dind]`](config.md#keys) for the config reference:

```toml
[run.dind]
enabled = true
```

The Docker host must allow unprivileged user namespaces (the default on Docker Desktop and most Linux distributions). If the sidecar fails to start, the error includes its logs.

## How it stays isolated

- The tool container keeps every restriction listed under [Security model](overview.md#security-model); it only gains `DOCKER_HOST` and a client cert.
- The daemon runs as your user (like the tool container), inside a user namespace, never `--privileged`. Files that inner containers write to `/workspace` as root are owned by you. Every other inner-container id maps to a dedicated range starting at 2,000,000,000 that no host account owns, not to your own subuid range. Running agentic as root with `--dind` is refused. To create that namespace the sidecar (only) gets namespace-scoped capabilities, an unconfined AppArmor profile, and `/dev/net/tun`.
- The sidecar keeps a seccomp filter: Docker's default profile, minus kernel interfaces nested containers don't need (`bpf`, `perf_event_open`, `syslog`, ...). Keyring syscalls stay blocked.
- The sidecar image (`agentic-dind`) is built locally from the upstream rootless image with every setuid/setgid bit removed, so nothing in it can become root. The id-map helpers get only `CAP_SETUID`/`CAP_SETGID`. It is rebuilt with a fresh base pull at least weekly.
- The tool reaches the daemon over mutual TLS with throwaway per-run certs. The CA key never touches disk, and containers started through the daemon can't drive it unless they bind-mount the daemon's socket or certs.
- The daemon only sees `/workspace` (with any `read_only_mounts` under it still read-only) - not the tool home, secrets, or the rest of your filesystem.
- Images, containers, and volumes are discarded when the run ends. Per-run certs and Docker config (including any `docker login` credentials) are removed too. If a run crashed, the next `agentic` run removes its sidecar, data, and certs once they are a few minutes old, and `agentic clean` removes them right away. Published ports are reachable from the tool at `agentic-docker:<port>`, never from the host.
- With `--proxy`, the sidecar shares the tool's internal network, so pulls, builds, and containers only reach the outside through the allowlist. Docker Hub is allowlisted automatically; add other registries (e.g. `ghcr.io`, `mcr.microsoft.com`) to `allowed_hosts`. The proxy filters by host only, so an allowlisted registry can also be pushed to.

## Trade-offs

- The agent can start containers with namespace-scoped `CAP_SYS_ADMIN`/`CAP_NET_ADMIN`, which exposes kernel interfaces (e.g. nf_tables, mounts, nested user namespaces) that a plain container can't reach. Isolation then rests on the host kernel's user-namespace code, so keep the Docker host's kernel patched.
- The sidecar has its own `--pids-limit`/`--cpus`/`--memory`, on top of the tool's, shared by everything the daemon runs. They default to the tool's limits (so a run can use up to twice them); set `[run.dind]` `pids_limit`/`cpus`/`memory` or `--dind-pids-limit`/`--dind-cpus`/`--dind-memory` to size the daemon separately. Images and volumes live on the Docker host's disk with no size limit.

See [Security model](security-model.md) for how this fits with the other layers and what risk is left.

See [Devcontainers](recipes.md#devcontainers) for testing devcontainers through the sidecar.
