# Microsandbox runtime

By default each tool runs in a Docker container, which shares the host's kernel. The microsandbox runtime runs it in a [microsandbox](https://docs.microsandbox.dev) (`msb`) microVM instead, with its own kernel, so a kernel exploit inside it doesn't reach your machine. Images are still built with Docker and loaded into msb.

## Requirements

- Docker, to build the images.
- msb 0.7.8 or newer on your `PATH`. Older versions are refused.
- Hardware virtualization: KVM on Linux, Hypervisor.framework on macOS.
- On WSL 2, nested virtualization, so KVM works inside it. See [WSL 2](#wsl-2).

### WSL 2

Turn on nested virtualization in `%UserProfile%\.wslconfig` (Windows 11 only):

```ini
[wsl2]
nestedVirtualization=true
```

Then restart WSL with `wsl --shutdown`. Afterwards `/dev/kvm` should exist inside WSL.

## Usage

```bash
agentic run --runtime microsandbox claude
```

Or set it per project, so every run there uses it:

```toml
[run]
runtime = "microsandbox"
```

`--runtime docker` overrides the config for one run. Changing `runtime` in a config file needs your [approval](config.md#setting-approval) on the next run, like other keys that change how far the tool reaches.

The first run of an image copies it from Docker into msb, which takes a while. Later runs reuse the copy until you rebuild or update the image, and then older copies are removed. `agentic clean` removes the msb copies as well.

## What's different from Docker

- **Filesystem**: the VM's root is writable but discarded when it exits, so there's no read-only root and no separate temp dirs. The workspace, tool home and `--volume` mounts work as with Docker.
- **Docker-in-Docker**: the daemon runs inside the VM, not in a sidecar. See [Docker in the VM](#docker-in-the-vm).
- **Egress allowlist**: msb's own network policy enforces it, with no access log and no `--proxy-monitor`. See [With microsandbox](egress-proxy.md#with-microsandbox).
- **Credential injection**: msb swaps [`[[run.proxy.credentials]]`](egress-proxy.md#credential-injection) secrets in for its own placeholder, without the proxy. Git over HTTPS sends no credentials.
- **Secret mounts**: `--secret` and `[run] secrets` are refused. Use credential injection instead.
- **Named volumes**: msb stores them as `agentic-<name>`. See [Named volumes](usage.md#named-volumes).
- **Limits**: `cpus` (rounded up to a whole number) and `memory` apply to the VM. `pids_limit` is ignored.

## Docker in the VM

`--dind` and `[run.dind] enabled` start a Docker daemon inside the tool's own VM. No `--base docker` is needed: the first such run builds `<image>-dockerd`, the tool image plus Docker's daemon and CLI, and copies that into msb instead. It's rebuilt after the tool image changes, and `agentic clean` removes it with the tool image.

- **Root in the VM**: dockerd runs as root, so the tool can reach root in the VM through it. The VM stays the boundary: msb enforces the allowlist, read-only mounts and credentials from the host side.
- **Storage**: images, containers and volumes live on a private 20G disk that's deleted when the VM exits.
- **Limits**: the daemon shares the VM's `cpus` and `memory`. `[run.dind]` limits and `--dind-*` flags are ignored with a warning.
- **Ports**: published ports are on `localhost` in the VM, not on your machine.
- **With the allowlist**: msb inspects HTTPS with its own CA, which containers don't trust. Mount `/etc/ssl/certs/ca-certificates.crt` read-only into a container that needs HTTPS. `docker build` steps that fetch over HTTPS need that CA added in the Dockerfile.

## Limitations

- msb writes the terminal output of each run to its `exec.log`, so a secret printed to the terminal ends up on disk.
- A VM escape lands in msb's process on the host, which runs as your user. See [Security model](security-model.md).
