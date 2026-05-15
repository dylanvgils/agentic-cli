# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Bash + Docker framework for running agentic coding tools (Claude Code, Copilot, OpenCode) in sandboxed containers. No test suite, no linter. Development means editing shell scripts and Dockerfiles, then testing by building and running.

## Key commands

```bash
agentic build [tool] [--base java|dotnet] [--no-cache]
agentic update [tool] [--base java|dotnet] [--no-cache]
agentic clean [tool]
agentic inspect [tool]
agentic <tool> [args]
```

## Code conventions

### Shell scripts
- All executable scripts: `#!/usr/bin/env bash` + `set -euo pipefail`
- Sourced files (not executed directly): no shebang, no `set -euo pipefail`, start with a comment saying so
- `shellcheck source=` annotations on every `source` line
- `local` for all function-scoped variables; declare and assign on separate lines when the value comes from a subshell (so `set -e` catches errors)
- Arrays for multi-value Docker args (`DOCKER_ARGS=()`, `MOUNTS+=(-v ...)`, `TMPFS_FLAGS=(...)`)
- Section headers as `# --- Section Name ---` comments

### Tool structure
Each tool in `tools/<name>/` must implement exactly: `config.sh`, `build.sh`, `update.sh`, `Dockerfile`, `entrypoint.sh`. Tools are discovered dynamically by scanning for `config.sh` - no registration needed anywhere.

`config.sh` sets `BASE`, `IMAGE`, `VERSION_CMD` and sources `shared/config.sh` (which sets `TOOL_HOME`).

`build.sh` and `update.sh` are one-liners: source `config.sh` + call the shared function.

Tool execution is handled entirely by the Go CLI (`agentic-cli run <tool>`). Tool-specific mount configuration and setup live in `internal/tools/<tool>.go`.

### Shared scripts
`shared/scripts/` scripts are sourced, not executed. They expose functions for building images (`build-common.sh`, `repo-root.sh`).

### Adding a new runtime layer
Drop a `Dockerfile` in `shared/base/<name>/`. It must accept `BASE_IMAGE` as a build arg. The build system derives the version env var as `AGENTIC_<NAME>_VERSION` automatically.

### Go tests
- Use Arrange-Act-Assert (AAA) in every test: `// Arrange`, `// Act`, `// Assert` comment labels with a blank line between sections
- Omit `// Arrange` only when there is genuinely nothing to set up
- Use `// Act + Assert` only when a single call is inseparably both (e.g. `assert.Panics`)
- Assign the result of the function under test to a variable in `// Act` so `// Assert` can reference it — do not inline the call inside the assertion

### Security constraints (enforced in `internal/docker/run.go`)
`--read-only`, `--cap-drop=ALL`, `--security-opt=no-new-privileges:true`, `--user $(id -u):$(id -g)`. Do not relax these. If a tool needs write access, use a targeted tmpfs or volume mount instead.

### Keeping docs in sync
Any change that affects user-facing behaviour must be reflected in both:
- `README.md` - user documentation (commands, flags, config, examples)
- `bin/agentic` - the usage string in the `usage()` function

### Mount handling
`CONTAINER_HOME` is resolved at runtime from the image's `TOOL_HOME` env var via `docker.ResolveContainerHome` in `internal/docker/inspect.go`. Mount strings support two placeholders expanded by `docker.ExpandMountVars` in `internal/docker/run.go` before the `docker run` call:
- `$TOOL_HOME` / `${TOOL_HOME}` - host-side agentic data dir; use on the left (host path) side of `:`
- `$CONTAINER_HOME` / `${CONTAINER_HOME}` - container home dir; use on the right (container path) side of `:`

Always use single quotes or escape `$` when passing mount strings through the shell.
