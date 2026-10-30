# CLAUDE.md

Go CLI (`agentic`, `cmd/cli`, Cobra tree in `internal/cli`) that runs agentic coding tools in hardened containers. Dockerfiles are generated from Go (`internal/image/dockerfile` DSL); no static Dockerfiles exist.

```bash
make build              # bin/agentic; CLI changes need this, stage changes also need `agentic build`
make test               # unit tests
make lint               # golangci-lint, same as CI
make audit              # go mod verify + govulncheck, in CI and nightly
make test-integration   # against a real Docker daemon
make verify-checksums   # pinned install-script checksums (fix-checksums to update)
```

## Import boundaries

A new package goes in the layer group of what it imports.

- Layers, lowest first: `internal/infra` < `internal/runtime/sandbox` < `internal/config` < `internal/image`, `internal/network` (never each other) < `internal/runtime` < `internal/usecase` < `internal/cli`. A layer never imports a higher one; from `internal/runtime`, `config` and `image` import only `runtime/sandbox`, the contract the runtimes share.
- `agentic-proxy` (`cmd/proxy`) imports only `internal/network/proxy`, never `internal/runtime/docker`, `tools`, `cli` or `logging`.
- `internal/runtime/dind` (sidecar files) never imports `internal/runtime/docker`; the sidecar's container lifecycle lives in `internal/runtime/docker/dind.go`.
- `internal/network/certs` imports only the standard library.
- `internal/network/proxy` never imports `internal/network/credentials`.

## Security rules

- The tool container keeps `--read-only`, `--cap-drop=ALL`, `no-new-privileges`, the host uid/gid and an isolated network (`internal/runtime/docker/runargs.go`). Need writes? Add a targeted ephemeral dir or volume.
- Only the DinD sidecar gets extra capabilities, unconfined AppArmor and no `no-new-privileges`. Its seccomp filter is never `unconfined`. Never `--privileged`, never mount the host's Docker socket.
- With `--proxy` the tool's network is `--internal`, so the proxy is its only way out.
- A new `.agenticrc.toml` key goes in `GuardedSettings` if it reaches past the container, else in the unguarded list of `TestGuardedSettings`, which fails until it's classified. A run reads config only through the `approvedConfig` that `checkSettings` returns.
- `Service.Run` rechecks mounts and refuses a `RunSpec` needing a capability the runtime lacks, for every runtime; runners keep their own rechecks too.
- Tool volumes come only from `newMountSet` (`internal/usecase/run/mounts.go`), which refuses mounts through workspace symlinks or exposing `agentic.json` or `state.json`. Never add mounts to the `RunSpec` around it.

## Architecture

- `internal/cli` is presentation only: parse flags, call domain packages, print. Logic moves to `internal/usecase/<what-it-does>` only when it's independent of `*cobra.Command` and glues several domain packages together; no package per command.
- `sandbox.RunSpec` describes the run the user asked for; each runtime (`docker`, `microsandbox`) turns it into its own args and keeps per-run state private (docker's `toolRun`).
- All Docker calls go through `*docker.Client`; no package-level Docker funcs or state. Client methods are operations; deciding what results mean belongs to the caller. `internal/cli` holds the one client in `dockerClient` (rebuilt in `persistentPreRunE`) behind its `dockerAPI` interface.
- A usecase that calls Docker declares a `Docker` interface of only the methods it uses in `deps.go` (`var _ Docker = (*docker.Client)(nil)` in `deps_test.go`) and holds it in `type Service struct{ docker Docker }` from `New(d)`, plus `home string` if it touches the data dir. Other runtime clients (microsandbox) follow the same pattern: an interface in `deps.go`, a `Service` field, a `New` param, one client in `internal/cli/deps.go`. Exported entry points are `Service` methods; unexported helpers only if they call Docker. Callers name it `svc`.
- A runtime's run steps live in its runner (`internal/usecase/run/<runtime>.go`, interface in `runtime.go`), which holds only the clients it uses: `Check` refuses a request before any prompt, `Run` runs it. A new runtime adds a runner and a case in `Service.runner`; code outside the runners reads `Capabilities()` instead of comparing runtime names, except for the VM wording in the instructions.
- Docker runs in phases: `StartSidecars` returns a `Sidecars` handle, then `RunTool` and `StopSidecars`, so the runner can recheck mounts between them.
- No new package-level func vars as test seams. Call the func directly and test its real effect when that's cheap (files, temp dirs); otherwise inject it (`Service` field, interface, param). Package-level state only at the Cobra boundary (`dockerClient`, `msbClient`). Remove an existing seam when you touch its code. User approval goes through an interface or func the cli implements; usecases never read stdin.
- New tool: a `Configs` entry in `internal/image/tools/tools.go` plus `internal/image/tools/<name>.go`, using `claude` as the template. Its `Stage` builds `FROM prevStage`; `EphemeralDirs` includes at least `/tmp`; `MarketplaceMount` is nil if unsupported. Reuse `createContainerUser`, `aptInstallRun`, `sandbox.VolumeMount`.
- New runtime layer: a case in `extraStage()` and names in `knownExtras`/`LayerFlagDesc` (`bases.go`), a default in `Versions`/`ForLayer`/`versions.json`, and any base-stage apt packages in `layerPackages`. The `--<name>` flag registers itself.

## Code style

- File order: imports (stdlib, then others), consts, vars, types (by dependency), then constructor + methods per type (exported first), then standalone funcs (exported first, strictly by export status).
- Put a function in the file that owns its subject, not next to its caller, even if it has side effects. Move it (and its tests) only when its subject changes.
- Comments: short, one line unless more is genuinely needed. Blank lines between logical blocks.
- Every package has a package comment: `// Package x ...`, or `// Command x ...` in a `main` package. Lint (staticcheck ST1000) enforces it, except for `main` packages.
- Cobra `init()`: `AddCommand`, then the command's own flags, then shared flag helpers.
- DSL: install steps use `df.Run{Blocks: ...}`, version check scripts `df.Heredoc`. One `df.Block` per logical phase; independent commands get their own block; a multi-line command is several `Lines` in one block; `Chain: true` for `&&`-chained lines. Pipelines need a `pipefail` `df.Shell` first.
- Output goes through `internal/infra/logging`, never `fmt.Fprint*(os.Stdout/Stderr)`. `Step`/`Detail` for items; action commands print one `Infof` summary first (single-object actions only that line), display commands none; during `agentic run` use `Infof`/`Warnf`/`Promptf`. Take a `*logging.Logger`, not a writer (tests pass `logging.New(&buf)`). Exception: dry-run `docker ...` lines stay plain.
- A failure that stops the operation is returned, not logged; only the entry points (`cli.Execute`, `cmd/proxy`) print it, once. One the operation survives gets a `Warnf` where it happens and isn't returned.
- Before adding a helper, grep for one that already computes it and reuse that; no second variant (sorted, filtered) beside it.
- Loop over a collection once: loops over the same input that each skip what the other handles become one loop with a branch.
- When a format changes (e.g. a comma list becomes JSON), drop the checks and comments that only guarded the old one.
- Ignored `Close`-style errors: `//nolint:errcheck` in tests, `defer func() { _ = x.Close() }()` elsewhere.
- Run `make lint` and `shellcheck` (fix, or disable inline with a reason) before committing.
- Use `-`, never em or en dashes, in all files.
- Mount strings: `$TOOL_HOME`, `~`, `$HOME`, `$PWD` on the host (left) side, `$CONTAINER_HOME` on the container (right) side. Single-quote them or escape `$` in the shell.

## Tests

- Always test new code, only the function's own responsibility (not its helpers' logic).
- AAA with `// Arrange`, `// Act`, `// Assert` and blank lines between. Any setup before the act gets `// Arrange`; shared setup at the top of the parent doesn't. `// Act + Assert` only for calls like `assert.Panics`.
- Assign the result in Act; don't call inside the assertion.
- Name `TestFunc` / `Test_func` (`Test_type_method` for methods). Several cases go in `t.Run` subtests with lowercase sentence names; a single case stays flat.
- Shared helpers live in `helpers_test.go`, register cleanup with `t.Cleanup`, and are named `stub...` when they stub.
- Docker tests use a hand-written `fakeDocker` (one func field per method; nil succeeds). In `internal/cli` use `stubDocker(t, &fakeDocker{...})`.
- Integration tests: `//go:build integration`, in the feature package under `test/integration/` (`container`, `network`, `proxy`, `lifecycle`, `tools`), whose `TestMain` calls `platform.Main`. Runs come from the `platform.ToolRun` builder, fixtures only from the `platform.FakeContainer`/`FakeNetwork` builders, whose `agentic.itest.run` label the end-of-run cleanup removes by. A helper goes in `platform` (plain `.go` files, so other packages can import it) only when several feature packages use it, else in that package's `helpers_test.go`. `make test-integration` builds the images once (`test/integration/cmd/buildimages`) and sets `AGENTIC_ITEST_IMAGES_BUILT` so `Main` skips them; a lone `go test` builds them itself. Packages run in parallel on one daemon, so a check never counts the daemon's containers; scope it to the run with `ToolRun.Containers`/`Proxy`.

## Docs

- User-facing changes update the matching `docs/` page. Touch `README.md` only for what it covers; a new page gets a line in `docs/README.md` and the README index.
- Each fact lives on one page; others link in one line. Grep `docs/` first.
- Paragraphs about 3 sentences, table cells one; bullets for conditions and steps.
- No internals users can't act on; contributor details go in `docs/development.md` or here.
- Never condense or rewrite personal sections such as Motivation in `docs/overview.md`.
