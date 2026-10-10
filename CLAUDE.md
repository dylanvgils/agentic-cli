# CLAUDE.md

Go CLI (`agentic`, `cmd/cli`, Cobra tree in `internal/cli`) that runs agentic coding tools in hardened containers. Dockerfiles are generated from Go (`internal/dockerfile` DSL); no static Dockerfiles exist.

```bash
make build              # bin/agentic; CLI changes need this, stage changes also need `agentic build`
make test               # unit tests
make lint               # golangci-lint, same as CI
make test-integration   # against a real Docker daemon
make verify-checksums   # pinned install-script checksums (fix-checksums to update)
```

## Import boundaries

- `agentic-proxy` (`cmd/proxy`) imports only `internal/proxy`, never `internal/docker`, `tools`, `cli` or `logging`.
- `internal/dind` (sidecar files) never imports `internal/docker`; the sidecar's container lifecycle lives in `internal/docker/dind.go`.
- `internal/certs` imports only the standard library.
- `internal/proxy` never imports `internal/credentials`.

## Security rules

- The tool container keeps `--read-only`, `--cap-drop=ALL`, `no-new-privileges`, the host uid/gid and an isolated network (`internal/docker/runargs.go`). Need writes? Add a targeted tmpfs or volume.
- Only the DinD sidecar gets extra capabilities, unconfined AppArmor and no `no-new-privileges`. Its seccomp filter is never `unconfined`. Never `--privileged`, never mount the host's Docker socket.
- With `--proxy` the tool's network is `--internal`, so the proxy is its only way out.
- Tool volumes and secrets come only from `newMountSet` (`internal/usecase/run/mounts.go`), which pins workspace symlinks and refuses escaping links or mounts exposing `agentic.json`. Never add mounts to the `RunSpec` around it.

## Architecture

- `internal/cli` is presentation only: parse flags, call domain packages, print. Logic moves to `internal/usecase/<what-it-does>` only when it's independent of `*cobra.Command` and glues several domain packages together; no package per command.
- All Docker calls go through `*docker.Client`; no package-level Docker funcs or state. Client methods are operations; deciding what results mean belongs to the caller. `internal/cli` holds the one client in `dockerClient` (rebuilt in `persistentPreRunE`) behind its `dockerAPI` interface.
- A usecase that calls Docker declares a `Docker` interface of only the methods it uses in `deps.go` (`var _ Docker = (*docker.Client)(nil)` in `deps_test.go`) and holds it in `type Service struct{ docker Docker }` from `New(d)`, plus `home string` if it touches the data dir. Exported entry points are `Service` methods; unexported helpers only if they call Docker. Callers name it `svc`.
- Non-Docker seams stay package-level vars in `deps.go`. User approval goes through an interface or func the cli implements; usecases never read stdin.
- New tool: a `Configs` entry in `internal/tools/tools.go` plus `internal/tools/<name>.go`, using `claude` as the template. Its `Stage` builds `FROM prevStage`; `TmpfsMounts` includes at least `/tmp`; `MarketplaceMount` is nil if unsupported. Reuse `createContainerUser`, `aptInstallRun`, `mount.VolumeMount`/`TmpfsMount`.
- New runtime layer: a case in `extraStage()` and names in `knownExtras`/`LayerFlagDesc` (`bases.go`), a default in `Versions`/`ForLayer`/`versions.json`, and any base-stage apt packages in `layerPackages`. The `--<name>` flag registers itself.

## Code style

- File order: imports (stdlib, then others), consts, vars, types (by dependency), then constructor + methods per type (exported first), then standalone funcs (exported first, strictly by export status).
- Put a function in the file that owns its subject, not next to its caller, even if it has side effects. Move it (and its tests) only when its subject changes.
- Comments: short, one line unless more is genuinely needed. Blank lines between logical blocks.
- Cobra `init()`: `AddCommand`, then the command's own flags, then shared flag helpers.
- DSL: install steps use `df.Run{Blocks: ...}`, version check scripts `df.Heredoc`. One `df.Block` per logical phase; independent commands get their own block; a multi-line command is several `Lines` in one block; `Chain: true` for `&&`-chained lines. Pipelines need a `pipefail` `df.Shell` first.
- Output goes through `internal/logging`, never `fmt.Fprint*(os.Stdout/Stderr)`. `Step`/`Detail` for items; action commands print one `Infof` summary first (single-object actions only that line), display commands none; during `agentic run` use `Infof`/`Warnf`/`Promptf`. Take a `*logging.Logger`, not a writer (tests pass `logging.New(&buf)`). Exception: dry-run `docker ...` lines stay plain.
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
- Integration tests: `test/integration/`, `//go:build integration`, fixtures from the `fakeContainer`/`fakeNetwork` builders in `fakeresource_test.go` (the one exception to `helpers_test.go`).

## Docs

- User-facing changes update the matching `docs/` page. Touch `README.md` only for what it covers; a new page gets a line in `docs/README.md` and the README index.
- Each fact lives on one page; others link in one line. Grep `docs/` first.
- Paragraphs about 3 sentences, table cells one; bullets for conditions and steps.
- No internals users can't act on; contributor details go in `docs/development.md` or here.
- Never condense or rewrite personal sections such as Motivation in `docs/overview.md`.
