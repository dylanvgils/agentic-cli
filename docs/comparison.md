# Comparison

How agentic compares to [Docker Sandboxes](https://docs.docker.com/ai/sandboxes/) (`sbx`), Docker's own way to isolate coding agents. It runs each agent in its own microVM - a stronger boundary than a container. The table reflects both as of October 2026; check Docker's docs for current details.

| | agentic | Docker Sandboxes |
| --- | --- | --- |
| **Boundary** | Container on the host's kernel | microVM with its own kernel |
| **Host requirements** | Any Docker-compatible runtime (Docker, Rancher Desktop, Podman) on any OS or architecture, no virtualization needed | macOS 14+ on Apple silicon, Windows 11 on x86_64, or Ubuntu 24.04+ on x86_64/arm64, with hardware virtualization |
| **Agent inside** | Non-root, read-only root filesystem, all capabilities dropped | `sudo` inside the VM; the VM is the boundary |
| **Network** | Open by default; opt-in allowlist proxy (`--proxy`) that logs every attempt | Deny by default; allowlist per machine or managed by your organization |
| **Credentials** | Opt-in: API keys injected by the egress proxy, the agent sees a placeholder; OAuth logins and mounted secrets stay readable | Injected by a host-side proxy; the agent never sees raw values |
| **Docker inside** | Opt-in rootless sidecar (`--dind`) | Private Docker Engine in every sandbox |
| **Config** | `.agenticrc.toml` per project, merged up the directory tree | `sbxenv.yaml` environment files (experimental) |
| **Account and cost** | None, free | Docker sign-in required, free |
| **Source** | Open source (MIT) | Published as binaries |

If your platform supports it and you want a VM boundary, or OAuth logins the agent can't read, Docker Sandboxes is a good fit. If you need portability, deep per-project configuration, or a setup you can fully audit, agentic is.
