# Development

Working on the CLI requires Go and Make installed locally.

## Repository structure

```
agentic-cli/
├── cmd/
│   ├── cli/                     # Thin entrypoint for the agentic binary (main.go only)
│   └── proxy/                   # Thin entrypoint for the agentic-proxy binary (main.go only)
├── internal/
│   ├── buildinfo/               # Build-time version/commit metadata and dev-build classification
│   ├── certs/                   # Per-run throwaway CAs and leaf certs (stdlib only, shared by dind and the proxy)
│   ├── cleanup/                 # Capture helper for propagating deferred cleanup errors without masking an earlier error
│   ├── cli/                     # Cobra commands (build, update, clean, inspect, run, …)
│   ├── config/                  # .agenticrc.toml loading and run spec
│   ├── credentials/             # Resolves [[run.proxy.credentials]] presets and secret files into the proxy's header rules (host side)
│   ├── dind/                    # Docker-in-Docker sidecar per-run files: TLS cert dirs, seccomp profile, /etc identity
│   ├── docker/                  # Build, update, run, clean, inspect, volume, and sidecar (proxy, dind) orchestration
│   ├── dockerfile/              # Dockerfile DSL (stages, instructions, builder)
│   ├── git/                     # Thin wrapper over the host git binary (CheckAvailable, Clone, FetchReset)
│   ├── housekeeping/            # Host-side cleanup not tied to a tool run, the proxy server, or docker orchestration (e.g. pruning proxy logs)
│   ├── marketplace/             # Syncs git-based plugin marketplace repos onto the host and tracks per-clone usage
│   ├── mount/                   # Volume mount spec builder
│   ├── output/                  # CLI output formatting
│   ├── platform/                # Platform-specific paths and utilities
│   ├── proxy/                   # Egress allowlist proxy: server, allowlist, JSON-lines logger, TLS-terminating credential injection
│   ├── selfupdate/              # Downloads and installs new releases from GitHub
│   ├── tools/                   # Per-tool stage funcs, mounts, setup, and base layers
│   └── usecase/                 # Business logic extracted out of internal/cli commands (see below)
│       ├── build/               # Builds (or dry-run prints) tool images for `agentic build`
│       ├── clean/               # Resolves and removes tool images and global Docker resources for `agentic clean`
│       ├── resolve/             # Merges CLI flags, .agenticrc.toml, and agentic.json into the effective value of every setting agentic supports
│       ├── run/                 # Builds docker.RunSpec for `agentic run` from resolved settings - marketplace sync, resource limits
│       ├── toolupdate/          # Checks for and applies upstream tool version updates on `agentic run`
│       ├── update/              # Resolves and applies `agentic update` targets - build-option recovery, --pull throttling
│       └── upgradecheck/        # Checks for and offers to apply a newer agentic CLI release, on any command's PersistentPreRunE
└── test/
    └── integration/             # Black-box tests driving the agentic binary against a real Docker daemon (`integration` build tag)
```

Package boundaries (what each sidecar package may import) are listed in [CLAUDE.md](../CLAUDE.md#what-this-is).

Dockerfiles are generated in Go, see [Dockerfile DSL](dockerfile-dsl.md).

## Build & test

```bash
make build          # compile to bin/agentic
make lint           # run golangci-lint
make test           # run unit tests
make test-integration # run integration tests against a real Docker daemon
make dist           # cross-platform binaries → dist/
make docker-dist    # same via Docker (no local Go needed)
```

CLI changes take effect after `make build`. Changes to stage funcs in `internal/tools/` or `internal/docker/` need an `agentic build` to rebuild the image.

Integration tests build the binary and an `agentic-itest-claude` image (slow on the first run, cached after), then drive `agentic run` like a user would. They skip without Docker, and the resource-limit checks skip without cgroups (e.g. rootless without systemd). CI runs them in a separate job. This repo's `.agenticrc.toml` enables DinD (and the proxy in monitor mode), so they also run inside an agentic container.

## Conventions

Code style, tests and lint rules are in [CLAUDE.md](../CLAUDE.md#code-conventions). Install `golangci-lint` at the version CI uses:

```bash
curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.12.2
# or: brew install golangci-lint
# or (slower, not recommended upstream): go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
```

## Adding a new tool

1. Create `internal/tools/<name>.go` implementing four functions:
   - `<name>Stage(prevStage string) dockerfile.Stage` - return the tool's Dockerfile stage using the [Dockerfile DSL](dockerfile-dsl.md); `prevStage` is the name of the preceding base stage to `FROM`
   - `setup<Name>(toolHome string) error` - create any host-side directories or files the tool needs before first run (e.g. pre-creating a credentials file so the read-only root filesystem doesn't block the first write)
   - `<name>Mounts() []string` - return the list of bind/volume mounts using helpers from `internal/mount`
   - `<name>TmpfsMounts() []string` - return any tmpfs mounts (every tool needs at least `/tmp`)

   Reuse the shared helpers in `internal/tools/helpers.go` inside the stage func:
   - `createContainerUser(name string) []df.Instruction` - declares `HOST_UID`/`HOST_GID` build args, removes any conflicting user, and creates the container user. Spread into `Add`: `Add(createContainerUser("mytool")...)`
   - `aptInstallRun(pkgs []string) df.Run` - builds a standard apt update → install → cleanup `RUN` block

   Use `mount.VolumeMount(host, container)` and `mount.TmpfsMount(path, opts)` from `internal/mount`, with the [mount placeholders](config.md#mount-variable-expansion). Never relax the security flags, see [CLAUDE.md](../CLAUDE.md#security-constraints-enforced-in-internaldockerrungo).

2. Register in `internal/tools/tools.go` `Configs` map:

   ```go
   "mytool": {
       Build:   BuildConfig{Stage: mytoolStage},
       Runtime: RuntimeConfig{TmpfsMounts: mytoolTmpfsMounts, Setup: setupMytool, Mounts: mytoolMounts, AllowedHosts: mytoolAllowedHosts},
   },
   ```

   `AllowedHosts` is the tool's [baseline allowlist](egress-proxy.md#baseline-allowlist). Define it as a package-level `var` in `internal/tools/<name>.go` (see the other tools for examples).

## Adding a new base runtime

1. Add a new case to `extraStage()` in `internal/tools/bases.go` (follow the `nodeStage`/`javaStage`/`dotnetStage`/`goStage` pattern). The stage func receives `prevStage` and `ver` - build FROM `prevStage` and apply the version as a build arg default.

2. Add the name to `knownExtras` in `internal/tools/bases.go` and add a human-readable label to `LayerFlagDesc` in the same file. The `--<name>` version flag is registered automatically from these two maps.

3. If the new layer needs apt packages installed in the base stage (e.g. `apt-transport-https` for Java), add them to `layerPackages` in `internal/tools/packages.go` under the layer's name. `collectPackages` merges them with the base packages and any user-supplied `--apt` packages automatically.

Each image stores its resolved versions and apt list as labels (`agentic.version-args`, `agentic.apt`, see `internal/docker/labels.go`). `agentic update` reads them back (`RecoverVersionArgs`, `RecoverApt`) to rebuild with the same layers.

## Docker-in-Docker sidecar image

`agentic run --dind` builds the global `agentic-dind` image lazily (`ensureDindImage`) from `tools.GenerateDindDockerfile` (see [Image](docker-in-docker.md#how-it-stays-isolated)). It always builds with `--pull` and is rebuilt when missing, built by another CLI version, or older than 7 days (`tools.DindImageMaxAge`). `agentic clean` removes it.

The sidecar's seccomp profile is derived at run time (`deriveSeccompProfile`) from Docker's default profile, vendored verbatim in `internal/dind/seccomp_default.json`. To refresh it, re-copy `seccomp/default.json` from [moby/profiles](https://github.com/moby/profiles) and update the commit noted on `seccompDefault`; `Test_deriveSeccompProfile` checks the derived rules still hold.

## Orphaned sidecars

Sidecars and their networks carry `agentic.owner` (the tool container name) and `agentic.started` labels. If the CLI is killed, its cleanup never runs, so each run starts with `sweepOrphanedSidecars` (`internal/docker/sidecar.go`). It removes sidecars whose owner is gone and that are older than `sidecarOrphanGrace` (5 minutes, which spares a run that is still starting).

## Building the proxy image locally

The global `agentic-proxy` image installs the `agentic-proxy` binary from `cmd/proxy`. `agentic build` never builds it; `ensureProxyImage` builds it on the first `--proxy` run.

Released builds `go install` the published `cmd/proxy` module at their own version. Local builds default `VERSION` to `dev`, which makes the proxy Dockerfile compile from the local source tree instead - detected by walking up from `$PWD` looking for the module's `go.mod`, so run these from the repository root:

```bash
make build                          # compile the CLI binary (version = "dev")
./bin/agentic run --proxy claude    # compiles the proxy from local source on first use
```

An existing image is never treated as stale. After editing code the proxy links in, rebuild it:

```bash
./bin/agentic proxy update    # always rebuilds agentic-proxy with --no-cache
```

## Releasing

When a PR merges to `main` and CI passes, `.github/scripts/next-tag.sh` tags a release if any commit since the last tag needs one, following [Conventional Commits](https://www.conventionalcommits.org/):

| Commit type                                                         | Bump       |
| ------------------------------------------------------------------- | ---------- |
| `feat!:`, `fix!:`, or any type with `!` / `BREAKING CHANGE:` footer | major      |
| `feat:`                                                             | minor      |
| `fix:`, `perf:`, `refactor:`                                        | patch      |
| `chore:`, `docs:`, `ci:`, `test:`, `style:`, `build:`               | no release |

Scoped variants (`feat(tool):`) count the same. Dry-run it locally:

```bash
.github/scripts/next-tag.sh          # next tag based on latest git tag
.github/scripts/next-tag.sh v0.3.0   # next tag based on a specific tag
```

## Debugging

Open a shell in the container with `agentic run <tool> -- bash` ([Usage](usage.md#examples)), then for example:

```bash
mount | grep -v "^cgroup\|^proc\|^tmpfs"   # what is mounted where
which claude && claude --version         # tool on PATH
env | sort                               # environment variables
```
