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
- `/home/agent/.copilot`: writable, persists across runs.
- Every other path, `/tmp` included, is writable but discarded when the VM exits.

## Resource limits

- CPUs: 3
- Memory: 6g

These are hard limits for this run. If a task needs more, tell the user so they can raise cpus/memory in .agenticrc.toml (or --cpus/--memory) and rerun.

## Privileges

- Non-root user. `sudo` and apt fail.

## Additional instructions

Always run go test before finishing.
