# Security model

A plain-language map of how agentic isolates a tool: each layer, what it stops, and what it doesn't. The threat is the agent itself - a bug, a bad decision, or a prompt injection in something it reads. The goal: it can only touch what you hand it.

## The map

```mermaid
flowchart LR
    subgraph host[Your machine]
        ws[(workspace)]
        th[(tool home)]
        sec[(secrets)]
        subgraph docker[Docker host - shared kernel]
            tool[Tool container]
            proxy[Egress proxy<br/>opt-in]
            dind[DinD sidecar<br/>opt-in]
            inner[Inner containers]
        end
    end
    net((Internet))

    ws -- rw --> tool
    th -- rw --> tool
    sec -- ro --> tool
    ws -- rw --> dind
    tool -- HTTP/S --> proxy -- allowlisted hosts --> net
    tool -- mTLS --> dind --> inner
    dind -.->|with --proxy| proxy
    tool -.->|without --proxy| net
    dind -.->|without --proxy| net
```

Everything inside "Docker host" shares one Linux kernel. That's the boundary all layers below ultimately rest on. Without `--proxy`, both the tool and the DinD sidecar (with its inner containers) reach the internet directly.

## Layers

> [!WARNING]
> The egress proxy, credential injection and DinD sidecar layers are **beta**: under active development and not yet tested as thoroughly as the core container hardening. Don't rely on them as your only safeguard yet.

| Layer | What it stops | What it doesn't stop | Code |
| --- | --- | --- | --- |
| **Tool container**: read-only root, all capabilities dropped, `no-new-privileges`, your uid/gid, pid/cpu/memory limits | Changing the system, becoming root, fork bombs and runaway memory | Anything it can do with the mounts it's given; a kernel bug | `internal/docker/runargs.go` |
| **Mounts**: only `/workspace`, the tool's own home dir, read-only secrets, and extra `-v`/`extra_mounts` bind mounts (read-only unless `:rw`) | Reading the rest of your files (`~/.ssh`, other repos, browser data) | Reading everything that *is* mounted, including its own credentials and secrets; editing the workspace | `internal/tools/<tool>.go`, `internal/mount` |
| **Network** `agentic-net` | Reaching other containers on your Docker host | Reaching the internet | `internal/docker/network.go` |
| **Egress proxy** (opt-in, beta, `--proxy`): tool sits on an `--internal` network whose only way out is the proxy | Talking to hosts not on the allowlist; every attempt is logged | Sending data to an allowlisted host (filtering is by host, not content) | `internal/proxy`, `internal/docker/proxy.go` |
| **Credential injection** (opt-in, beta, `[[run.proxy.credentials]]`): the proxy holds API keys and sets them as headers for their hosts; the tool sees a placeholder | Reading or leaking a configured API key; containers outside the run's network (e.g. another agentic run) using it | Using the key through its hosts during the run; OAuth tokens in the tool home; Copilot's exchanged token | `internal/proxy`, `internal/credentials`, `internal/usecase/run/credentials.go` |
| **DinD sidecar** (opt-in, beta, `--dind`): rootless dockerd in its own container, mTLS, seccomp filter, no setuid binaries, sees only `/workspace` | Access to the host's Docker socket (= root on your machine); host files outside `/workspace` | Inner containers get namespaced `SYS_ADMIN`/`NET_ADMIN`, so more kernel surface; an inner container can read the sidecar's own files (incl. its TLS key); direct internet access without `--proxy`; `docker push` to an allowlisted registry | `internal/docker/dind.go`, `internal/dind` - see [Docker-in-Docker](docker-in-docker.md) |
| **Build**: tool install scripts checksum-verified against a pinned SHA256 | A tampered or swapped install script | A malicious release of the tool itself | `internal/tools` |

## Turning layers on

- Egress proxy and credential injection: `--proxy`, `--proxy-monitor`, `[[run.proxy.credentials]]`. See [Egress proxy](egress-proxy.md).
- DinD sidecar: `--dind`. See [Docker-in-Docker](docker-in-docker.md).
- Read-only sub-paths: [`read_only_mounts`](config.md#read_only_mounts).

Install scripts for Claude, Copilot and OpenCode are checked against a pinned SHA256 before they run, not piped straight into `bash`. A daily job opens a PR when an upstream script changes. If a build fails on a stale checksum before that PR lands, `--skip-install-checksum` skips the check for that build.

## Rules that must never break

If one of these breaks, a layer above stops meaning anything:

- Never mount the host's Docker socket.
- Never `--privileged`, never relax the tool container's flags.
- Extra capabilities, unconfined AppArmor and no `no-new-privileges` are for the DinD sidecar only.
- The sidecar's seccomp filter is never `unconfined`.
- The sidecar only sees `/workspace`.

## Leftover risk

What no layer covers today:

- **Shared kernel**: a kernel exploit escapes every container. Only a VM boundary (Kata, gVisor, [Docker Sandboxes](comparison.md)) fixes this. Keep the host kernel patched.
- **Credentials in reach**: the agent can read its OAuth login token in the tool home and any `--secret` you mount, and use them from any allowed host. [Proxy-injected API keys](egress-proxy.md#credential-injection) stay out of reach, but can still be used.
- **Workspace tampering**: the agent can edit git hooks, `Makefile`, `package.json` scripts, etc. They run on *your* machine the next time you use them outside the container. Review diffs.
- **Exfiltration to allowed hosts**: without `--proxy` the internet is open to the tool and the DinD sidecar; with it, data can still go to any allowlisted host.

## Check it yourself

Run these inside the tool container:

```bash
touch /etc/test                    # fails: Read-only file system
grep CapEff /proc/self/status      # 0000000000000000: no capabilities
grep NoNewPrivs /proc/self/status  # 1: can't gain privileges
ls ~/.ssh                          # fails: not mounted
curl -I https://example.com        # with --proxy and not allowlisted: CONNECT tunnel failed, response 403
```

## Glossary

- **Capability**: a slice of root's power (e.g. `CHOWN`, `NET_ADMIN`). Dropping all of them means even uid 0 can do little.
- **Namespace**: the kernel giving a process its own view of something (processes, network, mounts, users). Containers are mostly namespaces.
- **User namespace**: maps "root inside" to an unprivileged uid outside. Rootless Docker is built on this.
- **Rootless**: dockerd runs as your user instead of root, so its "root" is fake.
- **seccomp**: a list of syscalls a process may make; everything else is refused by the kernel.
- **AppArmor**: a kernel policy limiting which files and actions a program may use.
- **`no-new-privileges`**: blocks gaining privileges through setuid binaries or file capabilities.
- **mTLS**: both sides prove who they are with certificates, so only the tool can drive the sidecar's daemon.
