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

No static Dockerfile files exist. All Dockerfiles are generated at build time by composing `dockerfile.Stage` values from `internal/tools/bases.go` (base and extra layers) and each tool's `Stage` func. See [dockerfile-dsl.md](dockerfile-dsl.md) for the DSL reference.

## Build & test

```bash
make build          # compile to bin/agentic
make test           # run unit tests
make dist           # cross-platform binaries → dist/
make docker-dist    # same via Docker (no local Go needed)
```

Changes to the CLI take effect immediately after `make build` - no container rebuild needed. Changes to stage funcs in `internal/tools/` or `internal/docker/` require an `agentic build` to rebuild the affected image.

## Adding a new tool

1. Create `internal/tools/<name>.go` implementing four functions:
   - `<name>Stage(prevStage string) dockerfile.Stage` — return the tool's Dockerfile stage using the [Dockerfile DSL](dockerfile-dsl.md); `prevStage` is the name of the preceding base stage to `FROM`
   - `setup<Name>(toolHome string) error` — create any host-side directories or files the tool needs before first run (e.g. pre-creating a credentials file so the read-only root filesystem doesn't block the first write)
   - `<name>Mounts() []string` — return the list of bind/volume mounts using helpers from `internal/mount`
   - `<name>TmpfsMounts() []string` — return any tmpfs mounts (every tool needs at least `/tmp`)

   Reuse the shared helpers in `internal/tools/helpers.go` inside the stage func:
   - `CreateContainerUser(name string) []df.Instruction` — declares `HOST_UID`/`HOST_GID` build args, removes any conflicting user, and creates the container user. Spread into `Add`: `Add(CreateContainerUser("mytool")...)`
   - `AptInstallRun(pkgs []string) df.Run` — builds a standard apt update → install → cleanup `RUN` block

   Use `mount.VolumeMount(host, container)` and `mount.TmpfsMount(path, opts)` from `internal/mount`. Mount strings support two placeholder variables expanded at runtime:
   - `$TOOL_HOME` (host side) — expands to the agentic data dir (e.g. `~/.agentic`)
   - `$CONTAINER_HOME` (container side) — expands to the container home dir, resolved from the image's `TOOL_HOME` env var

   Security constraints (`--read-only`, `--cap-drop=ALL`, `--security-opt=no-new-privileges:true`) are enforced in `internal/docker/run.go`. Do not relax them. If the tool needs to write somewhere, use a targeted tmpfs or volume mount — not a relaxed security flag.

2. Register in `internal/tools/tools.go` `Configs` map:

   ```go
   "mytool": {
       Build:   BuildConfig{Stage: mytoolStage},
       Runtime: RuntimeConfig{TmpfsMounts: mytoolTmpfsMounts, Setup: setupMytool, Mounts: mytoolMounts},
   },
   ```

## Adding a new base runtime

1. Add a new case to `ExtraStage()` in `internal/tools/bases.go` (follow the `javaStage`/`dotnetStage`/`goStage` pattern). The stage func receives `prevStage` and `ver` — build FROM `prevStage` and apply the version as a build arg default.

2. Add the name to `KnownExtras` in `internal/tools/bases.go`.

3. Wire a `--<name>` version flag into three files:
   - `cmd/flags.go` — define the flag
   - `cmd/build.go` — pass it through to the build step
   - `cmd/update.go` — pass it through to the update step

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
