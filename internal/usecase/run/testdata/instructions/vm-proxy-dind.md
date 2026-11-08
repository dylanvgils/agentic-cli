# Agentic microVM environment

You are running inside an agentic-cli microVM with its own kernel. The sections below list what's installed and what's blocked, so you don't try things that fail.

## Precedence

agentic-cli writes this block on every run and leaves the rest of this file as is. It describes the environment only. If it conflicts with the project's own instructions file (CLAUDE.md, AGENTS.md, copilot-instructions.md), follow the project's file.

## Installed toolchains

- Base toolchain (always installed):
  - curl
  - wget
  - git
  - gpg
  - ca-certificates
  - tzdata
  - jq
  - python3
- Extra runtimes:
  - node@24.2.0
  - java@21.0.1
- Additional apt packages:
  - make
  - gcc
- Custom installs:
  - helm

Only the tools above and the standard Debian utilities (coreutils, `grep`, `sed`, `tar`, ...) are installed. Any other tool isn't, so check with `command -v <tool>` before relying on it. Package managers work only when they install into a writable path (see Filesystem) from a reachable registry, e.g. `npm install` in `/workspace`. For any other missing tool, tell the user which tool you need and why, so they can add it to custom_installs in .agenticrc.toml and rebuild the image.

## Filesystem

- `/workspace`: the project on the user's machine. Writable; changes are real.
- `/home/agent/.opencode`: writable, persists across runs.
- `/home/agent/.local/share/opencode`: writable, persists across runs.
- `/home/agent/.local/state/opencode`: writable, persists across runs.
- `/home/agent/.cache/opencode`: writable, persists across runs.
- `/home/agent/.config/opencode`: writable, persists across runs.
- Every other path, `/tmp` included, is writable but discarded when the VM exits.

## Resource limits

- CPUs: 4
- Memory: 4g

These are hard limits for this run. If a task needs more, tell the user so they can raise cpus/memory in .agenticrc.toml (or --cpus/--memory) and rerun.

## Privileges

- Non-root user. `sudo` and apt fail.

## Network

- The VM's network policy filters all traffic and inspects HTTPS.
- Only these hosts are reachable, and only on ports 80 and 443 (HTTP and HTTPS):
  - opencode.ai
  - registry-1.docker.io
  - auth.docker.io
  - production.cloudflare.docker.com
  - production.cloudfront.docker.com
  - extra.example.test
  - raw.example.test
- Everything else is blocked: other hosts, other ports and SSH, git over SSH included. A blocked request fails with a DNS, connection or TLS error.
- Web searches and page fetches (any search or fetch tool, `curl`, `wget`) only reach the hosts above.
- A blocked host is blocked for every tool. Don't retry it another way (curl, another fetch tool, a mirror).
- If a task needs a blocked host, tell the user which host and why, so they can add it to allowed_hosts in .agenticrc.toml and rerun.

## Docker

- `docker` uses a daemon that runs as root in this VM, with access to any path in it. No setup needed.
- Published ports are on `localhost` in the VM. The user's machine can't reach them.
- Images, containers and volumes live on a 20G disk and are deleted when this session ends.
- The daemon shares the VM's CPUs and memory (see Resource limits).
- Image pulls, builds and containers use the same allowlist; a registry other than Docker Hub must be added to allowed_hosts.
- Containers don't trust the VM's HTTPS inspection CA. For HTTPS inside a container, mount it: `-v /etc/ssl/certs/ca-certificates.crt:/etc/ssl/certs/ca-certificates.crt:ro`. `docker build` steps that use HTTPS fail unless the Dockerfile adds that CA.

