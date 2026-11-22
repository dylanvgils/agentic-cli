---
paths:
  - "internal/usecase/**"
  - "internal/runtime/sandbox/**"
---
# Usecase rules

- Usecase shape (follow `internal/usecase/build`):
  - `deps.go` declares an interface per client with only the methods it uses; `deps_test.go` checks it with `var _`.
  - `type Service struct` holds them from `New`, plus `home string` if it touches the data dir.
  - Exported entry points are `Service` methods; unexported helpers only if they call a client.
  - An action command's usecase has `Resolve(...) (Plan, error)`, then `DryRun(plan)` or `Apply(plan)`; the cli prints the summary between them.
  - `DryRun` only where there's a command or Dockerfile to print.
  - No Plan for `prune`, which finds and removes its targets in one runtime call so the live-run checks happen at removal, or `volume rm`, which acts on named volumes, the cli confirming `--all`.

## Run usecase

- Runners (`internal/usecase/run/<runtime>.go`, interface in `runtime.go`) hold a runtime's run steps:
  - `Check` refuses a request before any prompt, `Run` runs it, `DryRun` prints what `Run` would run.
  - A new runtime adds a runner and a case in `Service.runner`.
  - Read `Capabilities()` instead of comparing runtime names: `Features` are what a run can need, `Isolation` how the runtime isolates (VM wording uses `Boundary`).
- Docker runs in phases (`StartSidecars`, `RunTool`, `StopSidecars`) so the runner can recheck mounts between them.
- A `RunSpec` setting a runtime can't honor gets a `Features` field and an entry in `featureRules` (`usecase/run/capabilities.go`).
