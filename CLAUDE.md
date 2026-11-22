# CLAUDE.md

Go CLI (`agentic`, `cmd/cli`, Cobra tree in `internal/cli`) that runs agentic coding tools in hardened containers. Dockerfiles are generated from Go (`internal/image/dockerfile` DSL); there are no static Dockerfiles.

```bash
make build              # bin/agentic
make test               # unit tests
make lint               # golangci-lint
make audit              # go mod verify + govulncheck
make test-integration   # against a real Docker daemon; PKGS="egress proxy" for some, MSB=1 to require msb
make test-integration-sandbox # inside an agentic run: skips images and microsandbox, which can't pass there
make verify-checksums   # pinned artifact checksums, nvm and msb (fix-checksums to update)
```

CLI changes need `make build`; image stage changes also need `agentic build`.

## Import boundaries

- Layers, lowest first; a layer never imports a higher one:
  1. `internal/infra`
  2. `internal/runtime/sandbox`
  3. `internal/config`
  4. `internal/image`, `internal/network` (never each other)
  5. `internal/runtime` (`docker`, `microsandbox`, `dind`)
  6. `internal/usecase`
  7. `internal/cli`
- A new package goes in the layer of what it imports.
- Within a layer:
  - `runtime/dind` (sidecar files) never imports `runtime/docker`; the sidecar's container lifecycle lives in `runtime/docker/dind.go`.
  - `runtime/docker` and `runtime/microsandbox` never import each other.
  - Config, lowest first: `config/agenticrc` (`.agenticrc.toml`) < `config/agentichome` (`$AGENTIC_HOME` files) < `config/precedence`.
  - Usecases import no other usecase (only `migrate` its own `migrate/migrations`); they get others injected through `deps.go`.
  - `network/certs` imports only the stdlib.
- The proxy (`internal/network/proxy`, `cmd/proxy`) is a standalone service:
  - Non-test code imports only the stdlib, `golang.org/x/sys/unix`, `infra` (never `logging` or `selfupdate`) and `network/certs`; `cmd/proxy` only `infra/iostreams` and `network/proxy`.
  - All settings, listen address included, come from `AGENTIC_PROXY_*` env vars.
  - `cmd/proxy/main_test.go` keeps the CLI's packages out transitively.
- `depguard` in `.golangci.yml` enforces all of the above except `network/certs`: `layer-*` rules for the layers, one rule per package rule, strict allowlists for the proxy. A new layer or package rule needs a rule there too.

## Security rules

- The tool container keeps `--read-only`, `--cap-drop=ALL`, `no-new-privileges`, the host uid/gid and an isolated network (`internal/runtime/docker/runargs.go`). Need writes? Add a targeted ephemeral dir or volume.
- Only the DinD sidecar gets extra capabilities, unconfined AppArmor and no `no-new-privileges`. Its seccomp filter is never `unconfined`. Never `--privileged`, never mount the host's Docker socket.
- With the proxy enabled (`RunSpec.Egress.Mode` not off), the tool's network is `--internal`, so the proxy is its only way out.
- A new `.agenticrc.toml` key goes in `agenticrc.Config.GuardedSettings` if it reaches past the container, else in the unguarded list of `TestConfig_GuardedSettings`. A run reads config only through the `approvedConfig` that `checkSettings` returns.
- A new `.agenticrc.toml` key also needs merge, schema, `agentic config` and `docs/config.md`; `TestSchema_matchesConfig`, `Test_mergeConfigs_everyKey`, `Test_projectSections_everyKey` and `TestConfigDocs_everyKey` catch a miss.
- Tool volumes come only from `newMountSet` (`internal/usecase/run/mounts.go`); never add mounts to the `RunSpec` around it. `RunSpec.Mounts` are typed `sandbox.Mount`s, parsed once from the `-v` syntax by `sandbox.ParseMount` (`ParseUserMount` for user mounts); runtimes only render them. `run.Service.Apply` refuses a `RunSpec` that needs a capability the runtime lacks; each runner rechecks the mounts right before a process that mounts them starts, nowhere else.

## Architecture

- `internal/cli` is presentation only: parse flags, call domain packages, print.
- Flag-over-config precedence lives in `internal/config/precedence` (run settings) and `tools.BuildOptionsFor` (build options); pure functions, no clients.
- Logic moves to `internal/usecase/<what-it-does>` only when it's independent of `*cobra.Command` and glues several domain packages together. No package per command.
- User approval goes through an interface or func the cli implements; usecases never read stdin.
- A usecase that needs another usecase declares an interface for it in its `deps.go` and takes it in `New`; the cli builds the real one. Never build another usecase's `Service` or embed its `Docker` interface.
- `sandbox.RunSpec` describes the run the user asked for; each runtime (`docker`, `microsandbox`) turns it into its own args and keeps per-run state private.
  - It says what a run gets (`Egress`, `Credentials`, `Dind`), never how: sidecar names, aliases, env wiring and host log paths stay in the runtime.
  - Only what images bake in (the proxy's alias and port, the Maven settings path) lives in `internal/image/tools`.
  - A setting a runtime can't honor gets a `Features` field and an entry in `featureRules` (`usecase/run/capabilities.go`).
- Log files go in a dated dir from `logging.Dir` (a run passes its ID); `logging.Prune` removes whole dirs past the retention window.
- All Docker calls go through `*docker.Client`; no package-level Docker funcs or state. Client methods are operations; the caller decides what results mean.
- The only package-level state is the clients in `internal/cli/deps.go` (`dockerClient`, `msbClient`, ...), passed to usecases through `New`, and the defaults in `iostreams`/`logging`.
- Usecase shape (follow `internal/usecase/build`):
  - `deps.go` declares an interface per client with only the methods it uses; `deps_test.go` checks it with `var _`.
  - `type Service struct` holds them from `New`, plus `home string` if it touches the data dir.
  - Exported entry points are `Service` methods; unexported helpers only if they call a client.
  - An action command's usecase has `Resolve(...) (Plan, error)`, then `DryRun(plan)` or `Apply(plan)`; the cli prints the summary between them.
  - `DryRun` only where there's a command or Dockerfile to print.
  - No Plan for `prune`, which finds and removes its targets in one runtime call so the live-run checks happen at removal, or `volume rm`, which acts on named volumes, the cli confirming `--all`.
- Runners (`internal/usecase/run/<runtime>.go`, interface in `runtime.go`) hold a runtime's run steps:
  - `Check` refuses a request before any prompt, `Run` runs it, `DryRun` prints what `Run` would run.
  - A new runtime adds a runner and a case in `Service.runner`.
  - Read `Capabilities()` instead of comparing runtime names: `Features` are what a run can need, `Isolation` how the runtime isolates (VM wording uses `Boundary`).
- Docker runs in phases (`StartSidecars`, `RunTool`, `StopSidecars`) so the runner can recheck mounts between them.
- Test seams:
  - No new package-level func vars as seams.
  - Call the func directly and test its real effect when that's cheap (files, temp dirs); otherwise inject it (`Service` field, interface, param).
  - Remove an existing seam when you touch its code.
- New tool:
  - A `Configs` entry in `internal/image/tools/tools.go`.
  - `internal/image/tools/<name>.go`, with `claude` as the template.
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
  3. Constructor + methods per type, exported first; `funcorder` enforces the type part (it sees `New<Type>`, not a bare `New`).
  4. Standalone funcs (exported first, strictly by export status).
- Put a function in the file that owns its subject, not next to its caller, even if it has side effects. Move it (and its tests) only when its subject changes.
- Comments are short, one line unless more is genuinely needed. `main` packages get a `// Command x ...` comment; ST1000 enforces `// Package x ...` elsewhere.
- Cobra `init()`: `AddCommand`, then the command's own flags, then shared flag helpers.
- DSL:
  - Install steps use `df.Run{Blocks: ...}`, version check scripts `df.Heredoc`.
  - One `df.Block` per logical phase; independent commands get their own block.
  - A multi-line command is several `Lines` in one block; `Chain: true` for `&&`-chained lines.
  - Pipelines need `pipefailShell` first.
- Before adding a helper, grep for one that already computes it and reuse it; no second variant (sorted, filtered) beside it.
- Loop over a collection once: two loops over the same input that each skip what the other handles become one loop with a branch.
- When a format changes (e.g. a comma list becomes JSON), drop the checks and comments that only guarded the old one.
- Host files and dirs come from an `infra/hostfs` class (`Data`, `Private`, `Shared`), never `os.Mkdir*`/`os.WriteFile`; `OpenFile` and `os.Root` modes use its `Dir()`/`File()`. `forbidigo` and `gosec` enforce it.
- Ignored `Close`-style errors: `//nolint:errcheck` in tests, `defer func() { _ = x.Close() }()` elsewhere.
- Run `make lint` and `shellcheck` (fix, or disable inline with a reason) before committing.
- Use `-`, never em or en dashes, in all files.
- Mount strings: `$TOOL_HOME`, `~`, `$HOME`, `$PWD` on the host (left) side, `$CONTAINER_HOME` on the container (right) side. Single-quote them or escape `$` in the shell.

## Output and errors

- All IO goes through two packages, `slog` style with package funcs on `Default()`:
  - `internal/infra/iostreams`: a `Streams` with `In`, `Out`, `Err` (data and raw writers).
  - `internal/infra/logging`: a `Logger` writing every message to a `Streams`' `Err`.
- Only `iostreams.System()` names `os.Stdin`/`Stdout`/`Stderr`; the mains make it the default with `iostreams.SetDefault`. `forbidigo` enforces it.
- Streams:
  - Data goes to stdout through `iostreams.Print`/`Printf`/`Println`, never through `logging`, so a future level or quiet mode can't touch piped or `eval`ed output (aliases, dry-run, tables).
  - Use the raw `iostreams.Default().Out` only where a writer is needed (tables, subprocesses, Cobra), and `iostreams.Default().Err` for raw stderr (subprocess progress, Cobra).
  - Messages (`Step`, `Detail`, `Info`, `Warn`, `Error`, `Prompt`) go to stderr through the `logging` funcs.
  - Dry-run output uses `sandbox.PrintDryRun`/`PrintDryRunText`, from one `DryRun` path per client, never an `if dryRun` in a real one.
- The proxy is a service, not a CLI, and never uses `logging`. Everything it logs (requests, warnings, errors, net/http `ErrorLog`) goes through one `log/slog` logger, `proxy.NewLogHandler` on `iostreams` Out, which its main makes the slog default.
- Every print and message func has a plain and an `f` form: plain for a fixed string, `f` only when formatting. No `\n` in the text: a blank line is `iostreams.Println()` on stdout, `logging.Blank()` on stderr.
- Which message:
  - `Step`/`Detail` for items.
  - Action commands print one `Info` summary first (single-object actions only that line); display commands none.
  - During `agentic run`, or before another command's output (the upgrade check), use `Info`/`Warn`/`Prompt`.
- Use the package funcs where the IO happens. Take a `*logging.Logger` or `*iostreams.Streams` only when code needs its own streams (a TUI, concurrent runs), and a raw writer param only for output that goes elsewhere (a buffer, a pipe).
- A failure that stops the operation is returned, not logged; only the entry points (`cli.Execute`, `cmd/proxy`) print it, once. A failure the operation survives gets a `Warn` where it happens and isn't returned.

## Tests

- Always test new code, but only the function's own responsibility, not its helpers' logic:
  - A caller shows in one case that it uses a helper; it doesn't repeat the helper's or a lower package's cases.
  - Merge subtests that only check different effects of one code path.
- AAA with `// Arrange`, `// Act`, `// Assert` and blank lines between:
  - Any setup before the act gets `// Arrange`; shared setup at the top of the parent doesn't.
  - `// Act + Assert` only for calls like `assert.Panics`.
  - Assign the result in Act; don't call inside the assertion.
- Name `TestFunc` / `Test_func` (`TestType_Method` / `Test_type_method` for methods). Several cases go in `t.Run` subtests with lowercase sentence names; a single case stays flat.
- Subtests that differ only in inputs and expected values go in a table (`for _, tc := range []struct{...}`) with `t.Run(tc.name, ...)` per case and AAA in the loop body. Prefer exact `Equal`/`EqualError` over several `Contains`.
- Shared helpers live in `helpers_test.go`, register cleanup with `t.Cleanup`, and are named `stub...` when they stub.
- Docker tests use a hand-written `fakeDocker` (a func field per method a test stubs; a nil field or a method without one succeeds). In `internal/cli` use `stubDocker(t, &fakeDocker{...})`.
- IO tests stub the default streams only through `iostreamstest` (`StubIn`, `StubOut`, `StubErr`, `StubCombined`, `CaptureOut`, `CaptureErr`), never with a local copy. A `*logging.Logger` param gets `logging.New(iostreams.New(...))` over buffers.

### Integration tests

Setup and cleanup are in `docs/development.md`.

- `//go:build integration`, in the feature package under `test/integration/` (`container`, `egress`, `images`, `lifecycle`, `microsandbox`, `network`, `proxy`).
- Runs come from the `platform.ToolRun` builder (standalone proxy runs from `proxy`'s `proxyRun`), fixtures only from the `platform.FakeContainer`/`FakeNetwork` builders.
- `egress` only tests what needs `agentic run` (tool env, CA trust, client access, run summary), looping over `platform.Runtimes`; what the proxy does on its own goes in `proxy`, against `FakeContainer.ServingUpstream`.
- `TestMain` picks the images a package needs: `platform.Main` (proxy + test tool), `SmokeMain` (every tool, listed in `smokePackages`), `RuntimesMain` (adds microsandbox when msb is there), `ProxyMain` (no agentic run).
- A helper goes in `platform` (plain `.go` files) only when several feature packages use it, else in that package's `helpers_test.go`.
- Packages run in parallel on one daemon and one msb image store:
  - A check never counts the daemon's containers; scope it to the run with `ToolRun.Containers`/`Proxy`.
  - Only `cmd/cleanup`, after every package, removes shared resources such as the msb image copies.

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
