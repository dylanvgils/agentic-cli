# CLAUDE.md

Go CLI (`agentic`, `cmd/cli`, Cobra tree in `internal/cli`) that runs agentic coding tools in hardened containers. Dockerfiles are generated from Go (`internal/image/dockerfile` DSL), never static.

```bash
make build              # bin/agentic
make test               # unit tests
make lint               # golangci-lint
make audit              # go mod verify + govulncheck
make test-integration   # real Docker daemon; PKGS="egress proxy" for some, MSB=1 to require msb
make test-integration-sandbox # inside an agentic run: skips images and microsandbox
make verify-checksums   # pinned artifact checksums (fix-checksums to update)
```

CLI changes need `make build`; image stage changes also need `agentic build`.

More rules load per path from `.claude/rules/` (`tools`, `usecase`, `tests`, `docs`). User-facing changes also update `docs/`.

## Import boundaries

All enforced by `depguard` in `.golangci.yml`, except `network/certs`; a new layer or package rule needs a rule there too.

- Layers, lowest first; never import a higher one: `infra` < `runtime/sandbox` < `config` < `image`, `network` (never each other) < `runtime` (`docker`, `microsandbox`, `dind`) < `usecase` < `cli`. A new package goes in the layer of what it imports.
- `runtime/dind` never imports `runtime/docker` (the sidecar's container lifecycle is in `runtime/docker/dind.go`); `runtime/docker` and `runtime/microsandbox` never import each other.
- Config: `config/agenticrc` < `config/agentichome` < `config/precedence`.
- Usecases import no other usecase (injected through `deps.go`); only `migrate` imports `migrate/migrations`.
- `network/certs` imports only the stdlib.
- The proxy (`internal/network/proxy`, `cmd/proxy`) is a standalone service:
  - Non-test code imports only the stdlib, `golang.org/x/sys/unix`, `infra` (not `logging`/`selfupdate`) and `network/certs`; `cmd/proxy` only `infra/iostreams` and `network/proxy`. `cmd/proxy/main_test.go` checks this transitively.
  - All settings, listen address included, come from `AGENTIC_PROXY_*` env vars.
  - It never uses `logging`: everything (requests, warnings, errors, net/http `ErrorLog`) goes through one `log/slog` logger, `proxy.NewLogHandler` on `iostreams` Out, made the slog default by its main.

## Security rules

- The tool container keeps `--read-only`, `--cap-drop=ALL`, `no-new-privileges`, the host uid/gid and an isolated network (`internal/runtime/docker/runargs.go`). Writes get a targeted ephemeral dir or volume.
- Only the DinD sidecar gets extra capabilities, unconfined AppArmor and no `no-new-privileges`; its seccomp filter is never `unconfined`. Never `--privileged`, never the host's Docker socket.
- With the proxy enabled (`RunSpec.Egress.Mode` not off), the tool's network is `--internal`: the proxy is its only way out.
- A new `.agenticrc.toml` key:
  - Goes in `agenticrc.Config.GuardedSettings` if it reaches past the container, else in the unguarded list of `TestConfig_GuardedSettings`.
  - Needs merge, schema, `agentic config` and `docs/config.md`; `TestSchema_matchesConfig`, `Test_mergeConfigs_everyKey`, `Test_projectSections_everyKey` and `TestConfigDocs_everyKey` catch a miss.
- A run reads config only through the `approvedConfig` that `checkSettings` returns.
- Tool volumes come only from `newMountSet` (`internal/usecase/run/mounts.go`), never added to the `RunSpec` around it. `RunSpec.Mounts` are `sandbox.Mount`s parsed once by `sandbox.ParseMount` (`ParseUserMount` for user mounts); runtimes only render them.
- `run.Service.Apply` refuses a `RunSpec` needing a capability the runtime lacks; each runner rechecks mounts right before a process that mounts them starts, nowhere else.

## Architecture

- `internal/cli` is presentation only: parse flags, call domain packages, print.
- Flag-over-config precedence: `internal/config/precedence` (run) and `tools.BuildOptionsFor` (build); pure functions, no clients.
- Logic goes in `internal/usecase/<what-it-does>` only when it's independent of `*cobra.Command` and glues several domain packages. No package per command.
- User approval goes through an interface or func the cli implements; usecases never read stdin.
- A usecase needing another declares an interface in its `deps.go` and takes it in `New`; the cli builds the real one. Never build another usecase's `Service` or embed its `Docker` interface.
- `sandbox.RunSpec` says what a run gets (`Egress`, `Credentials`, `Dind`), never how; each runtime turns it into its own args and keeps per-run state (sidecar names, aliases, env wiring, host log paths) private. Only what images bake in (Maven settings template and paths) and the proxy sidecar's alias and port live in `internal/image/tools`; shared run env such as the integration env lives in `runtime/sandbox`.
- All Docker calls go through `*docker.Client`, no package-level Docker funcs; its methods are operations, the caller decides what results mean.
- The only package-level state: the clients in `internal/cli/deps.go`, passed to usecases through `New`, and the `iostreams`/`logging` defaults.
- Test seams: no new package-level func vars. Call the func directly when its real effect is cheap to test (files, temp dirs), else inject it (`Service` field, interface, param). Remove an existing seam when you touch its code.

## Code style

- File order, blank lines between blocks: imports (stdlib first); consts, vars, types (by dependency); constructor + methods per type, exported first (`funcorder`, which sees `New<Type>`, not a bare `New`); standalone funcs, exported first.
- A function lives in the file that owns its subject, not next to its caller. Move it (and its tests) only when its subject changes.
- Comments are short, one line unless genuinely needed. `main` packages get `// Command x ...`; others `// Package x ...` (ST1000).
- Cobra `init()`: `AddCommand`, the command's own flags, then shared flag helpers.
- Grep for an existing helper before adding one; no second variant (sorted, filtered) beside it.
- Loop over a collection once: two loops that each skip what the other handles become one with a branch.
- When a format changes, drop the checks and comments that only guarded the old one.
- Host files and dirs come from an `infra/hostfs` class (`Data`, `Private`, `Shared`) with its `Dir()`/`File()` modes for `OpenFile`/`os.Root`, never `os.Mkdir*`/`os.WriteFile` (`forbidigo`, `gosec`).
- Ignored `Close` errors: `//nolint:errcheck` in tests, `defer func() { _ = x.Close() }()` elsewhere (`nolintlint`).
- Run `make lint` and `shellcheck` (fix, or disable inline with a reason) before committing.
- Use `-`, never em or en dashes, in all files.
- Mount strings: `$TOOL_HOME`, `~`, `$HOME`, `$PWD` on the host (left) side, `$CONTAINER_HOME` on the container (right) side; single-quote or escape `$` in the shell.
- Log files go in a dated dir from `logging.Dir` (a run passes its ID); `logging.Prune` removes whole dirs past retention.

## Output and errors

- All IO goes through `internal/infra/iostreams` (`Streams` with `In`, `Out`, `Err`) and `internal/infra/logging` (a `Logger` writing to a `Streams`' `Err`), `slog` style with package funcs on `Default()`. Only `iostreams.System()` names `os.Std*`; mains set it with `iostreams.SetDefault` (`forbidigo`).
- Data goes to stdout through `iostreams.Print`/`Printf`/`Println`, never `logging`, so piped or `eval`ed output stays untouched. Raw `iostreams.Default().Out`/`.Err` only where a writer is needed (tables, subprocesses, Cobra).
- Messages (`Step`, `Detail`, `Info`, `Warn`, `Error`, `Prompt`) go to stderr through `logging`.
- Dry-run output uses `sandbox.PrintDryRun`/`PrintDryRunText` from one `DryRun` path per client, never an `if dryRun` in a real one.
- Plain func for a fixed string, `f` form only when formatting. No `\n` in text: blank lines are `iostreams.Println()` / `logging.Blank()`.
- `Step`/`Detail` for items. Action commands print one `Info` summary first (single-object actions only that); display commands none. During `agentic run` or before another command's output, use `Info`/`Warn`/`Prompt`.
- Use the package funcs where the IO happens. Take a `*logging.Logger`/`*iostreams.Streams` only for own streams (a TUI, concurrent runs), a raw writer only for output going elsewhere (a buffer, a pipe).
- A failure that stops the operation is returned, not logged; only `cli.Execute` and `cmd/proxy` print it, once. A survived failure gets a `Warn` where it happens and isn't returned.

## Tests

- Always test new code; the test rules are in `.claude/rules/tests.md`.
