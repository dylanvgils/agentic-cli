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
- `/home/agent/.claude`: writable, persists across runs.
- `/home/agent/.claude.json`: writable, persists across runs.
- Every other path, `/tmp` included, is writable but discarded when the VM exits.

## Resource limits

- CPUs: 4
- Memory: 4g

These are hard limits for this run. If a task needs more, tell the user so they can raise cpus/memory in .agenticrc.toml (or --cpus/--memory) and rerun.

## Privileges

- Non-root user. `sudo` and apt fail.

## Forwarded ports

- The user's machine reaches only these ports:
  - `localhost:3000` on the user's machine -> port 3000 here
- A server must listen on `0.0.0.0`. On `127.0.0.1` the user gets an empty response.
- Once it serves, give the user the matching `localhost` URL from this list.
- If a task needs another port, tell the user its number, so they can add it with `--port` and rerun.
- To show a Docker container, publish it on a forwarded port. For nginx, which listens on 80: `docker run --detach --publish 3000:80 nginx`.

## Docker

- `docker` uses a daemon that runs as root in this VM, with access to any path in it. No setup needed.
- Published ports are on `localhost` in the VM. The user's machine reaches them only through a forwarded port (see Forwarded ports).
- Images, containers and volumes live on a 20G disk and are deleted when this session ends.
- The daemon shares the VM's CPUs and memory (see Resource limits).

