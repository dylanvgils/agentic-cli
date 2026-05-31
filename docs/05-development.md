# Development

Working on the CLI requires Go and Make installed locally.

## Repository structure

```
agentic-cli/
├── cmd/                         # Cobra commands (build, update, clean, inspect, run, …)
└── internal/
    ├── config/                  # .agenticrc loading and run spec
    ├── docker/                  # Build, update, run, clean, inspect, volume orchestration
    ├── dockerfile/              # Dockerfile DSL (stages, instructions, builder)
    ├── mount/                   # Volume mount spec builder
    ├── output/                  # CLI output formatting
    ├── platform/                # Platform-specific paths and utilities
    └── tools/                   # Per-tool stage funcs, mounts, setup, and base layers
```

No static Dockerfile files exist. All Dockerfiles are generated at build time by composing `dockerfile.Stage` values from `internal/tools/bases.go` (base and extra layers) and each tool's `Stage` func. See [04-dockerfile-dsl.md](04-dockerfile-dsl.md) for the DSL reference.

## Build & test

```bash
make build          # compile to bin/agentic
make test           # run unit tests
make dist           # cross-platform binaries → dist/
make docker-dist    # same via Docker (no local Go needed)
```

Changes to the CLI take effect immediately after `make build` - no container rebuild needed. Changes to stage funcs in `internal/tools/` or `internal/docker/` require an `agentic build` to rebuild the affected image.

## Go conventions

### File structure

Within each `.go` file, order elements as follows:

1. Package declaration
2. Import block - two groups separated by a blank line: stdlib, then everything else (alphabetical within each group)
3. Constants (`const` blocks)
4. Package-level variables (`var` blocks)
5. Type declarations (structs, interfaces) - ordered by dependency/importance
6. Constructors and methods - grouped with their type; constructor first, then exported methods, then unexported methods
7. Standalone functions - exported functions first, then unexported helpers

### Style

- Use blank lines between logical blocks within a function to aid readability (e.g. between groups of related `if` statements, between `switch` case groups)

### Tests

- Always add tests for new code
- Use Arrange-Act-Assert (AAA) with `// Arrange`, `// Act`, `// Assert` comment labels and a blank line between sections
- Omit `// Arrange` only when there is genuinely nothing to set up
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

## Adding a new tool

1. Create `internal/tools/<name>.go` implementing four functions:
   - `<name>Stage(prevStage string) dockerfile.Stage` - return the tool's Dockerfile stage using the [Dockerfile DSL](04-dockerfile-dsl.md); `prevStage` is the name of the preceding base stage to `FROM`
   - `setup<Name>(toolHome string) error` - create any host-side directories or files the tool needs before first run (e.g. pre-creating a credentials file so the read-only root filesystem doesn't block the first write)
   - `<name>Mounts() []string` - return the list of bind/volume mounts using helpers from `internal/mount`
   - `<name>TmpfsMounts() []string` - return any tmpfs mounts (every tool needs at least `/tmp`)

   Reuse the shared helpers in `internal/tools/helpers.go` inside the stage func:
   - `CreateContainerUser(name string) []df.Instruction` - declares `HOST_UID`/`HOST_GID` build args, removes any conflicting user, and creates the container user. Spread into `Add`: `Add(CreateContainerUser("mytool")...)`
   - `AptInstallRun(pkgs []string) df.Run` - builds a standard apt update → install → cleanup `RUN` block

   Use `mount.VolumeMount(host, container)` and `mount.TmpfsMount(path, opts)` from `internal/mount`. Mount strings support two placeholder variables expanded at runtime:
   - `$TOOL_HOME` (host side) - expands to the agentic data dir (e.g. `~/.agentic`)
   - `$CONTAINER_HOME` (container side) - expands to the container home dir, resolved from the image's `TOOL_HOME` env var

   Security constraints (`--read-only`, `--cap-drop=ALL`, `--security-opt=no-new-privileges:true`) are enforced in `internal/docker/run.go`. Do not relax them. If the tool needs to write somewhere, use a targeted tmpfs or volume mount - not a relaxed security flag.

2. Register in `internal/tools/tools.go` `Configs` map:

   ```go
   "mytool": {
       Build:   BuildConfig{Stage: mytoolStage},
       Runtime: RuntimeConfig{TmpfsMounts: mytoolTmpfsMounts, Setup: setupMytool, Mounts: mytoolMounts},
   },
   ```

## Adding a new base runtime

1. Add a new case to `ExtraStage()` in `internal/tools/bases.go` (follow the `javaStage`/`dotnetStage`/`goStage` pattern). The stage func receives `prevStage` and `ver` - build FROM `prevStage` and apply the version as a build arg default.

2. Add the name to `knownExtras` in `internal/tools/bases.go` and add a human-readable label to `LayerFlagDesc` in the same file. The `--<name>` version flag and its `AGENTIC_<NAME>_VERSION` env var are registered automatically from these two maps.

3. If the new layer needs apt packages installed in the base stage (e.g. `apt-transport-https` for Java), add them to `layerPackages` in `internal/tools/packages.go` under the layer's name. `collectPackages` merges them with the base packages and any user-supplied `--apt` packages automatically.

## Debugging

To get a shell inside a container instead of running the tool, use `--` to override the entrypoint:

```bash
agentic run claude -- bash
agentic run opencode -- bash
```

From there you can inspect the filesystem, check environment variables, or run the tool manually to see raw output. Some useful starting points:

```bash
# Check what's mounted and where
mount | grep -v "^cgroup\|^proc\|^tmpfs"

# Verify the tool is on PATH and check its version
which claude && claude --version

# Inspect environment variables (API keys, TOOL_HOME, etc.)
env | sort
```
