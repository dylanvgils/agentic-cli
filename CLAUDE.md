# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go CLI + Docker framework for running agentic coding tools (Claude Code, Copilot, OpenCode) in isolated containers. The Go binary (`agentic`) handles all commands and generates Dockerfiles programmatically at build time - no static Dockerfile files exist. No linter. Development means editing Go source, then testing with `go test ./...` and building/running containers.

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

Adding a new tool requires an entry in `internal/tools/tools.go Configs` (holds `VersionCmd`, `TmpfsMounts`, `Setup`, `Mounts`, and `Stage`) plus the corresponding `internal/tools/<name>.go` file implementing `Setup`, `Mounts`, `TmpfsMounts`, and a `<name>Stage(prevStage string) dockerfile.Stage` function.

Dockerfiles are generated at build time by composing `dockerfile.Stage` values from `internal/docker/bases.go` (base layers) and the tool's `Stage` func. The DSL lives in `internal/dockerfile/`. No static Dockerfile files exist.

Tool execution is handled entirely by the Go CLI (`agentic run <tool>`). Tool-specific mount configuration and setup live in `internal/tools/<tool>.go`.

### Adding a new runtime layer

Add a new case to `extraStage()` in `internal/tools/bases.go` (follow the `javaStage`/`dotnetStage`/`goStage` pattern), add the name to `knownExtras`, and add a `--<name>` flag to `cmd/build.go`, `cmd/update.go`, and `cmd/flags.go`.

### Dockerfile DSL (`internal/dockerfile`)

Install steps use `df.Run{Blocks: []df.Block{...}}`; version check scripts use `df.Heredoc`. Block conventions:

- **Block = logical group**: each `Block` is one phase of work; the renderer joins blocks with `&&`.
- **Independent commands** within a group each get their own `Block` (one `Lines` entry).
- **Multi-line single commands** (pipelines, subshells) use multiple `Lines` entries within one `Block` - the renderer joins them with ` \` continuation.
- **`&&`-chained commands** within a group use `&&` prefix on each continuation line (e.g. download + verify + install + cleanup as one block).
- Stages that use pipelines need `df.Shell{Cmd: []string{"/bin/bash", "-o", "pipefail", "-c"}}` before the `Run`.

### Cobra command init functions

Every `init()` in a `cmd/*.go` file must follow this order:

1. `rootCmd.AddCommand(xCmd)` - command registration
2. Command-specific flags declared inline (`xCmd.Flags()...`)
3. Calls to shared flag helpers (`addBuildFlags`, `addNamespaceFlag`, `addAllFlag`, etc.)

```go
func init() {
    rootCmd.AddCommand(buildCmd)

    buildCmd.Flags().Bool("no-cache", false, "disable Docker layer cache for a fully fresh build")

    addBuildFlags(buildCmd)
    addNamespaceFlag(buildCmd)
}
```

### Go style

- Use blank lines between logical blocks within a function to aid readability (e.g. between groups of related `if` statements, between `switch` case groups)

### File structure

Within each `.go` file, order elements as follows:

1. Package declaration
2. Import block - two groups separated by a blank line: stdlib, then everything else (alphabetical within each group)
3. Constants (`const` blocks)
4. Package-level variables (`var` blocks)
5. Type declarations (structs, interfaces) - ordered by dependency/importance
6. Constructors and methods - grouped with their type; constructor first, then exported methods, then unexported methods
7. Standalone functions - exported functions first, then unexported helpers

### Go tests

- Always add tests for new code
- Use Arrange-Act-Assert (AAA) in every test: `// Arrange`, `// Act`, `// Assert` comment labels with a blank line between sections
- Omit `// Arrange` only when there is genuinely nothing to set up - if a subtest contains any setup statement before the act (stub configuration, variable declarations, flag setting, etc.), even a single line, label it `// Arrange`
- Use `// Act + Assert` only when a single call is inseparably both (e.g. `assert.Panics`)
- Assign the result of the function under test to a variable in `// Act` so `// Assert` can reference it - do not inline the call inside the assertion
- When a function has multiple test cases, group them under a single parent function using `t.Run` subtests; name the parent after the function under test (e.g. `TestBuildImage`). A function with only one test case stays as a flat top-level function
- Subtest names use lowercase sentence style derived from the scenario (e.g. `"first arg is build"`, `"noCache adds no-cache flag"`)
- Place shared setup that applies to all subtests at the top of the parent function body, before the first `t.Run` call; subtests with no additional setup omit `// Arrange`
- Test helper functions that need cleanup must register it via `t.Cleanup` internally - do not return a restore/teardown func for callers to defer
- All shared test helpers live in `helpers_test.go` in the same package; do not define helpers inside individual test files
- Name all stub helpers with a `stub` prefix (e.g. `stubDockerRun`, `stubRunInteractive`); pure utilities that are not stubs are exempt (e.g. `argAfter`)

Example structure:

```go
func TestBuildImage(t *testing.T) {
    get := stubRunInteractive(t) // shared setup - no // Arrange label needed at subtest level

    t.Run("first arg is build", func(t *testing.T) {
        // Act
        err := buildImage(...)
        // Assert
        assert.Equal(t, "build", get()[0])
    })

    t.Run("noCache adds no-cache flag", func(t *testing.T) {
        // Arrange
        opts := tools.BuildOptions{NoCache: true}
        // Act
        err := buildImage(..., opts)
        // Assert
        assert.Contains(t, get(), "--no-cache")
    })
}
```

### Shell scripts

Always check shell scripts with `shellcheck` before committing. Fix all warnings unless there is a specific reason to suppress a rule (add an inline `# shellcheck disable=SCxxxx` comment with a brief reason).

### Security constraints (enforced in `internal/docker/run.go`)

`--read-only`, `--cap-drop=ALL`, `--security-opt=no-new-privileges:true`, `--user $(id -u):$(id -g)`. Do not relax these. If a tool needs write access, use a targeted tmpfs or volume mount instead.

### Keeping docs in sync

Any change that affects user-facing behaviour must be reflected in `README.md` (commands, flags, config, examples).

Use `-` (hyphen) in all file content, never `—` (em dash) or `–` (en dash).

### Mount handling

`CONTAINER_HOME` is resolved at runtime from the image's `TOOL_HOME` env var via `docker.ResolveContainerHome` in `internal/docker/inspect.go`. Mount strings support two placeholders expanded by `mount.ExpandMountSpec` / `mount.ExpandTmpfsSpec` in `internal/mount/volume.go` (called from `internal/docker/run.go`) before the `docker run` call:

- `$TOOL_HOME` / `${TOOL_HOME}` - host-side agentic data dir; use on the left (host path) side of `:`
- `$CONTAINER_HOME` / `${CONTAINER_HOME}` - container home dir; use on the right (container path) side of `:`

Always use single quotes or escape `$` when passing mount strings through the shell.
