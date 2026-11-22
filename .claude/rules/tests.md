---
paths:
  - "**/*_test.go"
  - "test/**"
---
# Test rules

- Always test new code, but only the function's own responsibility, not its helpers' logic:
  - A caller shows in one case that it uses a helper; it doesn't repeat the helper's or a lower package's cases.
  - Merge subtests that only check different effects of one code path.
- AAA with `// Arrange`, `// Act`, `// Assert` and blank lines between:
  - Any setup before the act gets `// Arrange`; shared setup at the top of the parent doesn't.
  - `// Act + Assert` only for calls like `assert.Panics`.
  - Assign the result in Act; don't call inside the assertion.
- Name `TestFunc` / `Test_func` (`TestType_Method` / `Test_type_method` for methods). Several cases go in `t.Run` subtests with lowercase sentence names; a single case stays flat.
- Subtests that differ only in inputs and expected values go in a table (`for _, tc := range []struct{...}`) with `t.Run(tc.name, ...)` per case and AAA in the loop body. Prefer exact `Equal`/`EqualError` over several `Contains`.
- Assertions use `require` for errors and `expected, actual` order (`testifylint` enforces it).
- Shared helpers live in `helpers_test.go`, register cleanup with `t.Cleanup`, and are named `stub...` when they stub.
- Docker tests use a hand-written `fakeDocker` (a func field per method a test stubs; a nil field or a method without one succeeds). In `internal/cli` use `stubDocker(t, &fakeDocker{...})`.
- IO tests stub the default streams only through `iostreamstest` (`StubIn`, `StubOut`, `StubErr`, `StubCombined`, `CaptureOut`, `CaptureErr`), never with a local copy. A `*logging.Logger` param gets `logging.New(iostreams.New(...))` over buffers.

## Integration tests

Setup and cleanup are in `docs/development.md`.

- `//go:build integration`, in the feature package under `test/integration/` (`container`, `egress`, `images`, `lifecycle`, `microsandbox`, `network`, `proxy`).
- Runs come from the `platform.ToolRun` builder (standalone proxy runs from `proxy`'s `proxyRun`), fixtures only from the `platform.FakeContainer`/`FakeNetwork` builders.
- `egress` only tests what needs `agentic run` (tool env, CA trust, client access, run summary), looping over `platform.Runtimes`; what the proxy does on its own goes in `proxy`, against `FakeContainer.ServingUpstream`.
- `TestMain` picks the images a package needs: `platform.Main` (proxy + test tool), `SmokeMain` (every tool, listed in `smokePackages`), `RuntimesMain` (adds microsandbox when msb is there), `ProxyMain` (no agentic run).
- A helper goes in `platform` (plain `.go` files) only when several feature packages use it, else in that package's `helpers_test.go`.
- Packages run in parallel on one daemon and one msb image store:
  - A check never counts the daemon's containers; scope it to the run with `ToolRun.Containers`/`Proxy`.
  - Only `cmd/cleanup`, after every package, removes shared resources such as the msb image copies.
