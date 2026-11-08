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

## Network

- All traffic goes through an egress proxy.
- Only these hosts are reachable, and only on ports 80 and 443 (HTTP and HTTPS):
  - .anthropic.com
  - .claude.ai
  - .claude.com
  - registry-1.docker.io
  - auth.docker.io
  - production.cloudflare.docker.com
  - production.cloudfront.docker.com
  - extra.example.test
  - raw.example.test, only paths matching /org/repo/** (`*` is one path segment, `**` the rest)
- Everything else is blocked: other hosts, other ports and SSH, git over SSH included. A blocked request gets HTTP 403 from the proxy (curl: `CONNECT tunnel failed, response 403`).
- Web search (`WebSearch`) runs on the model provider's side and reaches any site. Fetching a page (a fetch tool, `curl`, `wget`) only reaches the hosts above.
- A blocked host is blocked for every tool. Don't retry it another way (curl, another fetch tool, a mirror).
- If a task needs a blocked host, tell the user which host and why, so they can add it to allowed_hosts in .agenticrc.toml and rerun.
- Other paths on a host limited to some paths get HTTP 403 too; tell the user which path and why, so they can add it to allowed_paths.

## Docker

- `docker` uses a rootless daemon in a per-run sidecar. DOCKER_HOST and TLS certs are preset; don't change them.
- Bind mounts work only under `/workspace`, which has the same path in the daemon. Other paths, such as `/tmp` or your home, don't exist there.
- Published ports are on the sidecar: use `agentic-docker:<port>`, not `localhost`. The user's machine can't reach them.
- Images, containers and volumes are deleted when this session ends.
- Daemon limits, shared by all its containers: 1024 processes, 2 CPUs, 6g memory. To raise them, tell the user to set pids_limit/cpus/memory under [run.dind] in .agenticrc.toml (or --dind-pids-limit/--dind-cpus/--dind-memory).
- Image pulls, builds and containers use the same allowlist; a registry other than Docker Hub must be added to allowed_hosts.

