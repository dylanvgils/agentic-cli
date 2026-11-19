# Agentic container environment

You are running inside an agentic-cli container. The sections below list what's installed and what's blocked, so you don't try things that fail.

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
- `/tmp`: writable, cleared when the container exits.
- Every other path is read-only.

## Resource limits

- Max processes: 1024
- CPUs: 4
- Memory: 4g

These are hard limits for this run. If a task needs more, tell the user so they can raise pids_limit/cpus/memory in .agenticrc.toml (or --pids-limit/--cpus/--memory) and rerun.

## Privileges

- Non-root user, all Linux capabilities dropped, no privilege escalation.
- `sudo`, apt and binding ports below 1024 fail.

## Forwarded ports

- The user's machine reaches only these ports:
  - `localhost:3000` on the user's machine -> port 3000 here
  - `localhost:8080` on the user's machine -> port 5173 here
- A server must listen on `0.0.0.0`. On `127.0.0.1` the user gets an empty response.
- Once it serves, give the user the matching `localhost` URL from this list.
- If a task needs another port, tell the user its number, so they can add it with `--port` and rerun.
- To show a Docker container, publish it on the sidecar under a forwarded port's number (nginx: `docker run --detach --publish 3000:80 nginx`). Then start a relay in the background that listens on `0.0.0.0` on that port here and forwards to `agentic-docker` on the same port. Write it in `python3`, which is always installed.

## Docker

- `docker` uses a rootless daemon in a per-run sidecar. DOCKER_HOST and TLS certs are preset; don't change them.
- Bind mounts work only under `/workspace`, which has the same path in the daemon. Other paths, such as `/tmp` or your home, don't exist there.
- Published ports are on the sidecar: use `agentic-docker:<port>`, not `localhost`. The user's machine reaches them only through a forwarded port (see Forwarded ports).
- Images, containers and volumes are deleted when this session ends.
- Daemon limits, shared by all its containers: 1024 processes, 4 CPUs, 4g memory. To raise them, tell the user to set pids_limit/cpus/memory under [run.dind] in .agenticrc.toml (or --dind-pids-limit/--dind-cpus/--dind-memory).

