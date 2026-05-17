# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go CLI + Docker framework for running agentic coding tools (Claude Code, Copilot, OpenCode) in sandboxed containers. The Go binary (`agentic`) handles all commands. No linter. Development means editing Go source and Dockerfiles, then testing with `go test ./...` and building/running containers.

## Key commands

```bash
agentic build [tool] [--base java|dotnet] [--no-cache]
agentic update [tool] [--base java|dotnet] [--no-cache]
agentic clean [tool]
agentic inspect [tool]
agentic <tool> [args]
```

## Code conventions

### Tool structure
Each tool in `tools/<name>/` must implement exactly: `Dockerfile`, `entrypoint.sh`. Adding a new tool requires an entry in `internal/tools/tools.go Configs` (holds `VersionCmd`, `TmpfsExecTmp`, `Setup`, and `Mounts`) plus the corresponding `internal/tools/<name>.go` file implementing `Setup` and `Mounts`.

Build and update logic lives in Go: `internal/tools/<tool>.go` holds runtime config (`Base`, `VersionCmd`); `internal/docker/build.go` and `internal/docker/update.go` hold the orchestration.

Tool execution is handled entirely by the Go CLI (`agentic run <tool>`). Tool-specific mount configuration and setup live in `internal/tools/<tool>.go`.

### Adding a new runtime layer
Drop a `Dockerfile` in `tools/base/<name>/` (must accept `BASE_IMAGE` as a build arg) and add a `--<name>` flag to `cmd/build.go`, `cmd/update.go`, and `cmd/flags.go`. The `--base <name>` routing is derived from the directory; version pinning requires the flag.

### Go style
- Use blank lines between logical blocks within a function to aid readability (e.g. between groups of related `if` statements, between `switch` case groups)

### Go tests
- Always add tests for new code
- Use Arrange-Act-Assert (AAA) in every test: `// Arrange`, `// Act`, `// Assert` comment labels with a blank line between sections
- Omit `// Arrange` only when there is genuinely nothing to set up
- Use `// Act + Assert` only when a single call is inseparably both (e.g. `assert.Panics`)
- Assign the result of the function under test to a variable in `// Act` so `// Assert` can reference it — do not inline the call inside the assertion

### Security constraints (enforced in `internal/docker/run.go`)
`--read-only`, `--cap-drop=ALL`, `--security-opt=no-new-privileges:true`, `--user $(id -u):$(id -g)`. Do not relax these. If a tool needs write access, use a targeted tmpfs or volume mount instead.

### Keeping docs in sync
Any change that affects user-facing behaviour must be reflected in `README.md` (commands, flags, config, examples).

### Mount handling
`CONTAINER_HOME` is resolved at runtime from the image's `TOOL_HOME` env var via `docker.ResolveContainerHome` in `internal/docker/inspect.go`. Mount strings support two placeholders expanded by `docker.ExpandMountVars` in `internal/docker/run.go` before the `docker run` call:
- `$TOOL_HOME` / `${TOOL_HOME}` - host-side agentic data dir; use on the left (host path) side of `:`
- `$CONTAINER_HOME` / `${CONTAINER_HOME}` - container home dir; use on the right (container path) side of `:`

Always use single quotes or escape `$` when passing mount strings through the shell.
