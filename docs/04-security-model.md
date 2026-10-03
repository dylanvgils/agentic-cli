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
```

Everything inside "Docker host" shares one Linux kernel. That's the boundary all layers below ultimately rest on.

## Layers

| Layer | What it stops | What it doesn't stop | Code |
| --- | --- | --- | --- |
| **Tool container**: read-only root, all capabilities dropped, `no-new-privileges`, your uid/gid, pid/cpu/memory limits | Changing the system, becoming root, fork bombs and runaway memory | Anything it can do with the mounts it's given; a kernel bug | `internal/docker/runargs.go` |
| **Mounts**: only `/workspace`, the tool's own home dir, and read-only secrets | Reading the rest of your files (`~/.ssh`, other repos, browser data) | Reading everything that *is* mounted, including its own credentials and secrets; editing the workspace | `internal/tools/<tool>.go`, `internal/mount` |
| **Network** `agentic-net` | Reaching other containers on your Docker host | Reaching the internet | `internal/docker/network.go` |
| **Egress proxy** (opt-in, `--proxy`): tool sits on an `--internal` network whose only way out is the proxy | Talking to hosts not on the allowlist; every attempt is logged | Sending data to an allowlisted host (filtering is by host, not content) | `internal/proxy`, `internal/docker/proxy.go` |
| **DinD sidecar** (opt-in, `--dind`): rootless dockerd in its own container, mTLS, seccomp filter, no setuid binaries, sees only `/workspace` | Access to the host's Docker socket (= root on your machine); host files outside `/workspace` | Inner containers get namespaced `SYS_ADMIN`/`NET_ADMIN`, so more kernel surface; an inner container can read the sidecar's own files (incl. its TLS key); `docker push` to an allowlisted registry | `internal/docker/dind.go`, `internal/dind` - see [Docker-in-Docker](05-docker-in-docker.md) |
| **Build**: tool install scripts checksum-verified against a pinned SHA256 | A tampered or swapped install script | A malicious release of the tool itself | `internal/tools` |

## Rules that must never break

If one of these breaks, a layer above stops meaning anything:

- Never mount the host's Docker socket.
- Never `--privileged`, never relax the tool container's flags.
- Extra capabilities, unconfined AppArmor and no `no-new-privileges` are for the DinD sidecar only.
- The sidecar's seccomp filter is never `unconfined`.
- The sidecar only sees `/workspace`.

## Leftover risk

What no layer covers today:

- **Shared kernel**: a kernel exploit escapes every container. Only a VM boundary (Kata, gVisor, Docker Sandboxes) fixes this. Keep the host kernel patched.
- **Credentials in reach**: the agent can read its own API token and any `--secret` you mount, and use them from any allowed host.
- **Workspace tampering**: the agent can edit git hooks, `Makefile`, `package.json` scripts, etc. They run on *your* machine the next time you use them outside the container. Review diffs.
- **Exfiltration to allowed hosts**: without `--proxy` the internet is open; with it, data can still go to any allowlisted host.

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
