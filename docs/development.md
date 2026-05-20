# Development

Working on the CLI requires Go and Make installed locally.

## Repository structure

```
agentic-cli/
├── cmd/                         # Cobra commands (build, update, clean, inspect, run, …)
├── internal/
│   ├── config/                  # .agenticrc loading and run spec
│   ├── docker/                  # Build, update, run, clean, inspect, volume orchestration
│   ├── mount/                   # Volume mount spec builder
│   ├── output/                  # CLI output formatting
│   ├── platform/                # Platform-specific paths and utilities
│   └── tools/                   # Per-tool runtime config (mounts, setup, version cmd)
└── tools/
    ├── base/
    │   ├── node/Dockerfile      # Base Node.js image (root layer)
    │   ├── java/Dockerfile      # Base Java image (extends node)
    │   ├── dotnet/Dockerfile    # Base .NET image (extends node)
    │   └── go/Dockerfile        # Base Go image (extends node)
    ├── claude/
    │   ├── Dockerfile
    │   └── entrypoint.sh
    ├── copilot/
    │   ├── Dockerfile
    │   └── entrypoint.sh
    └── opencode/
        ├── Dockerfile
        └── entrypoint.sh
```

## Build & test

```bash
make build          # compile to bin/agentic
make test           # run unit tests
make dist           # cross-platform binaries → dist/
make docker-dist    # same via Docker (no local Go needed)
```

Changes to the CLI take effect immediately after `make build` - no container rebuild needed. Changes to Dockerfiles in `tools/` require an `agentic build` to rebuild the affected image.

## Adding a new tool

1. Create `tools/<name>/Dockerfile` and `tools/<name>/entrypoint.sh`.

2. Create `internal/tools/<name>.go` implementing three functions:
   - `setup<Name>(toolHome string) error` — create any host-side directories or files the tool needs before first run (e.g. pre-creating a credentials file so the read-only root filesystem doesn't block the first write)
   - `<name>Mounts() []string` — return the list of bind/volume mounts using helpers from `internal/mount`
   - `<name>TmpfsMounts() []string` — return any tmpfs mounts (every tool needs at least `/tmp`)

   Use `mount.VolumeMount(host, container)` and `mount.TmpfsMount(path, opts)` from `internal/mount`. Mount strings support two placeholder variables expanded at runtime:
   - `$TOOL_HOME` (host side) — expands to the agentic data dir (e.g. `~/.agentic`)
   - `$CONTAINER_HOME` (container side) — expands to the container home dir, resolved from the image's `TOOL_HOME` env var

   Security constraints (`--read-only`, `--cap-drop=ALL`, `--security-opt=no-new-privileges:true`) are enforced in `internal/docker/run.go`. Do not relax them. If the tool needs to write somewhere, use a targeted tmpfs or volume mount — not a relaxed security flag.

3. Register in `internal/tools/tools.go` `Configs` map:

   ```go
   "mytool": {
       VersionCmd:  "mytool --version",
       TmpfsMounts: mytoolTmpfsMounts,
       Setup:       setupMytool,
       Mounts:      mytoolMounts,
   },
   ```

## Adding a new base runtime

1. Create `tools/base/<name>/Dockerfile`. It must accept `BASE_IMAGE` as a build arg so the layering system can stack it on top of any existing layer:

   ```dockerfile
   ARG BASE_IMAGE=agentic-base
   FROM ${BASE_IMAGE}
   ```

2. Wire a `--<name>` version flag into three files:
   - `cmd/flags.go` — define the flag
   - `cmd/build.go` — pass it through to the build step
   - `cmd/update.go` — pass it through to the update step

   The `--base <name>` routing is derived from the directory name automatically. The version flag is only needed if you want `--<name> <version>` pinning to work.

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
