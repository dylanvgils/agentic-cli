# CLAUDE.md

Go CLI (`agentic`, `cmd/cli`, Cobra tree in `internal/cli`) that runs agentic coding tools in hardened containers. Dockerfiles are generated from Go (`internal/image/dockerfile` DSL); no static Dockerfiles exist.

```bash
make build              # bin/agentic
make test               # unit tests
make lint               # golangci-lint
make audit              # go mod verify + govulncheck
make test-integration   # against a real Docker daemon
make test-integration-msb # microsandbox package only, needs msb and KVM
make verify-checksums   # pinned artifact checksums, nvm and msb (fix-checksums to update)
```

CLI changes need `make build`; image stage changes also need `agentic build`.

## Import boundaries

- Layers, lowest first: `internal/infra` < `internal/runtime/sandbox` < `internal/config` < `internal/image`, `internal/network` (never each other) < `internal/runtime` < `internal/usecase` < `internal/cli`. A layer never imports a higher one; a new package goes in the layer of what it imports.
- `cmd/proxy` imports only `internal/network/proxy`, `internal/infra/iostreams` and `internal/infra/logging`; `cmd/proxy/main_test.go` lists the transitive bans.
- `internal/runtime/dind` (sidecar files) never imports `internal/runtime/docker`; the sidecar's container lifecycle lives in `internal/runtime/docker/dind.go`.
- `internal/network/certs` imports only the standard library.
- `internal/network/proxy` never imports `internal/network/credentials`.

## Security rules

- The tool container keeps `--read-only`, `--cap-drop=ALL`, `no-new-privileges`, the host uid/gid and an isolated network (`internal/runtime/docker/runargs.go`). Need writes? Add a targeted ephemeral dir or volume.
- Only the DinD sidecar gets extra capabilities, unconfined AppArmor and no `no-new-privileges`. Its seccomp filter is never `unconfined`. Never `--privileged`, never mount the host's Docker socket.
- With the proxy enabled (any mode but off) the tool's network is `--internal`, so the proxy is its only way out.
- A new `.agenticrc.toml` key goes in `GuardedSettings` if it reaches past the container, else in the unguarded list of `TestGuardedSettings`. A run reads config only through the `approvedConfig` that `checkSettings` returns.
- Tool volumes come only from `newMountSet` (`internal/usecase/run/mounts.go`); never add mounts to the `RunSpec` around it. `Service.Run` rechecks mounts and refuses a `RunSpec` needing a capability the runtime lacks; runners keep their own rechecks too.

## Architecture

- `internal/cli` is presentation only: parse flags, call domain packages, print. Logic moves to `internal/usecase/<what-it-does>` only when it's independent of `*cobra.Command` and glues several domain packages together; no package per command.
- User approval goes through an interface or func the cli implements; usecases never read stdin.
- `sandbox.RunSpec` describes the run the user asked for; each runtime (`docker`, `microsandbox`) turns it into its own args and keeps per-run state private.
- All Docker calls go through `*docker.Client`; no package-level Docker funcs or state. Client methods are operations; deciding what results mean belongs to the caller.
- The only package-level state is the clients in `internal/cli/deps.go` (`dockerClient`, `msbClient`, ...), which usecases get through `New`, and the defaults in `iostreams`/`logging`.
- Usecase shape (follow `internal/usecase/build`):
  - `deps.go` declares an interface per client with only the methods it uses; `deps_test.go` checks it with `var _`.
  - `type Service struct` holds them from `New`, plus `home string` if it touches the data dir.
  - Exported entry points are `Service` methods; unexported helpers only if they call a client.
- Runners (`internal/usecase/run/<runtime>.go`, interface in `runtime.go`) hold a runtime's run steps:
  - `Check` refuses a request before any prompt, `Run` runs it.
  - A new runtime adds a runner and a case in `Service.runner`.
  - In `usecase/run`, read `Capabilities()` instead of comparing runtime names, except for the VM wording in the instructions.
- Docker runs in phases (`StartSidecars`, `RunTool`, `StopSidecars`) so the runner can recheck mounts between them.
- Test seams:
  - No new package-level func vars as seams.
  - Call the func directly and test its real effect when that's cheap (files, temp dirs); otherwise inject it (`Service` field, interface, param).
  - Remove an existing seam when you touch its code.
- New tool:
  - A `Configs` entry in `internal/image/tools/tools.go`.
  - `internal/image/tools/<name>.go`, using `claude` as the template.
  - Reuse `createContainerUser`, `aptInstallRun`, `sandbox.VolumeMount`.
- New runtime layer:
  - A case in `extraStage()` and names in `knownExtras`/`LayerFlagDesc` (`bases.go`).
  - A `Versions` field with defaults in `ForLayer`/`versions.json`.
  - Any base-stage apt packages in `layerPackages`.
  - No `--<name>` flag; it registers itself.

## Code style

- File order, with blank lines between logical blocks:
  1. Imports (stdlib, then others).
  2. Consts, vars, types (by dependency).
  3. Constructor + methods per type (exported first).
  4. Standalone funcs (exported first, strictly by export status).
- Put a function in the file that owns its subject, not next to its caller, even if it has side effects. Move it (and its tests) only when its subject changes.
- Comments: short, one line unless more is genuinely needed. `main` packages get a `// Command x ...` comment (ST1000 enforces `// Package x ...` elsewhere).
- Cobra `init()`: `AddCommand`, then the command's own flags, then shared flag helpers.
- DSL:
  - Install steps use `df.Run{Blocks: ...}`, version check scripts `df.Heredoc`.
  - One `df.Block` per logical phase; independent commands get their own block.
  - A multi-line command is several `Lines` in one block; `Chain: true` for `&&`-chained lines.
  - Pipelines need `pipefailShell` first.
- Before adding a helper, grep for one that already computes it and reuse that; no second variant (sorted, filtered) beside it.
- Loop over a collection once: loops over the same input that each skip what the other handles become one loop with a branch.
- When a format changes (e.g. a comma list becomes JSON), drop the checks and comments that only guarded the old one.
- Ignored `Close`-style errors: `//nolint:errcheck` in tests, `defer func() { _ = x.Close() }()` elsewhere.
- Run `make lint` and `shellcheck` (fix, or disable inline with a reason) before committing.
- Use `-`, never em or en dashes, in all files.
- Mount strings: `$TOOL_HOME`, `~`, `$HOME`, `$PWD` on the host (left) side, `$CONTAINER_HOME` on the container (right) side. Single-quote them or escape `$` in the shell.

## Output and errors

- All IO goes through `internal/infra/iostreams` (a `Streams` with `In`, `Out`, `Err`: data and raw writers) and `internal/infra/logging` (a `Logger` writing every message to a `Streams`' `Err`), `slog` style with package funcs on `Default()`. Only `iostreams.System()` names `os.Stdin`/`Stdout`/`Stderr`, and the mains make it the default with `iostreams.SetDefault`; `forbidigo` enforces it.
- Streams:
  - Data goes to stdout through `iostreams.Print`/`Printf`/`Println`, never through `logging`, so a future level or quiet mode in `logging` can't touch piped or `eval`ed output (aliases, dry-run, tables). The raw `iostreams.Default().Out` only where a writer is needed (tables, subprocesses, Cobra).
  - Messages (`Step`, `Detail`, `Info`, `Warn`, `Error`, `Prompt`) go to stderr through the `logging` funcs.
  - Raw stderr (subprocess progress, Cobra) goes to `iostreams.Default().Err`; an API that wants a std `*log.Logger` (net/http `ErrorLog`) gets `logging.NewLogLogger()`.
  - Dry-run output uses `sandbox.PrintDryRun`/`PrintDryRunText`.
- Every print and message func has a plain and an `f` form: plain for a fixed string, `f` only when formatting. No `\n` in the text: a blank line is `iostreams.Println()` on stdout, `logging.Blank()` on stderr.
- Which message:
  - `Step`/`Detail` for items.
  - Action commands print one `Info` summary first (single-object actions only that line); display commands none.
  - During `agentic run`, or before another command's output (the upgrade check), use `Info`/`Warn`/`Prompt`.
- Use the package funcs (`iostreams` for data, `logging` for messages) where the IO happens. Take a `*logging.Logger` or `*iostreams.Streams` only when code needs its own streams (e.g. a TUI, concurrent runs); a raw writer param only for output that goes elsewhere (a buffer, a pipe).
- A failure that stops the operation is returned, not logged; only the entry points (`cli.Execute`, `cmd/proxy`) print it, once. One the operation survives gets a `Warn` where it happens and isn't returned.

## Tests

- Always test new code, only the function's own responsibility (not its helpers' logic).
- AAA with `// Arrange`, `// Act`, `// Assert` and blank lines between. Any setup before the act gets `// Arrange`; shared setup at the top of the parent doesn't. `// Act + Assert` only for calls like `assert.Panics`.
- Assign the result in Act; don't call inside the assertion.
- Name `TestFunc` / `Test_func` (`TestType_Method` / `Test_type_method` for methods). Several cases go in `t.Run` subtests with lowercase sentence names; a single case stays flat.
- Shared helpers live in `helpers_test.go`, register cleanup with `t.Cleanup`, and are named `stub...` when they stub.
- Docker tests use a hand-written `fakeDocker` (one func field per method; nil succeeds). In `internal/cli` use `stubDocker(t, &fakeDocker{...})`.
- IO tests stub the default streams only through `iostreamstest` (`StubIn`, `StubOut`, `StubErr`, `StubCombined`, `CaptureOut`, `CaptureErr`), never with a local copy; a `*logging.Logger` param gets `logging.New(iostreams.New(...))` over buffers.

### Integration tests

Setup and cleanup are in `docs/development.md`.

- `//go:build integration`, in the feature package under `test/integration/` (`container`, `images`, `lifecycle`, `microsandbox`, `network`, `proxy`).
- Runs come from the `platform.ToolRun` builder, fixtures only from the `platform.FakeContainer`/`FakeNetwork` builders.
- A helper goes in `platform` (plain `.go` files) only when several feature packages use it, else in that package's `helpers_test.go`.
- Packages run in parallel on one daemon, so a check never counts the daemon's containers; scope it to the run with `ToolRun.Containers`/`Proxy`.

## Docs

- User-facing changes update the matching `docs/` page. Touch `README.md` only for what it covers.
- A new page gets a line in:
  - `docs/README.md`.
  - The `README.md` index.
  - The `nav` in `zensical.toml`.
- Each fact lives on one page; others link in one line. Grep `docs/` first.
- Paragraphs about 3 sentences, table cells one; bullets for conditions and steps.
- No internals users can't act on; contributor details go in `docs/development.md` or here.
- Never condense or rewrite personal sections such as Motivation in `docs/overview.md`.
