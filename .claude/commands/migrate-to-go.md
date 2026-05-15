# /migrate-to-go

Migrate a bash command from `bin/agentic` to the Go CLI (`agentic-cli`), following the same pattern as the `inspect` and `aliases` commands.

## Usage

```
/migrate-to-go <command-name>
```

Example: `/migrate-to-go completion`

---

## Steps

### 1. Read the bash source

In `bin/agentic`, find and read:

- `usage_<cmd>()` — understand the flags, arguments, and help text
- `cmd_<cmd>()` — understand inputs, outputs, and what Docker/tool APIs it uses

Note which `tools.*` functions, `inspectImage`, or shell helpers it relies on.

### 2. Create `cmd/<command>.go`

Model it after `cmd/inspect.go` or `cmd/aliases.go`:

```go
package cmd

import (
    "github.com/dylanvgils/agentic-cli/internal/tools"
    "github.com/spf13/cobra"
)

func init() {
    rootCmd.AddCommand(<cmd>Cmd)
}

var <cmd>Cmd = &cobra.Command{
    Use:       "<cmd> [args]",
    Short:     "<one-line description>",
    Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
    ValidArgs: tools.Names(), // or a fixed []string if args are not tool names
    RunE:      run<Cmd>,
}

func run<Cmd>(_ *cobra.Command, args []string) error {
    // implementation
}
```

Key reusable pieces (all in `cmd/root.go` and `internal/`):

- `inspectImage` — package-level var wrapping `docker.InspectImage`; returns `nil, nil` when image not built
- `tools.Names()` — sorted tool name list
- `tools.ImageName(name)` — returns `("agentic-<name>", error)`

When calling `dockerRun` inside `internal/docker/`, use the `arg()` helper (defined in `internal/docker/args.go`) for all Docker flags:

```go
arg("label", "project=agentic-cli")  // --label=project=agentic-cli
arg("filter", "label=project=agentic-cli")  // --filter=label=project=agentic-cli
arg("quiet")  // --quiet
arg("format", `{{index .Labels "project"}}`)  // --format={{index .Labels "project"}}
```

Do **not** pass flags as two separate strings (`"--flag"`, `"value"`) — always use `arg()` for consistency with the rest of `internal/docker/`.

### 3. Create `cmd/<command>_test.go`

Use `package cmd` to access internal helpers. Reuse from `inspect_test.go`:

- `captureStdout(t, fn)` — captures what `fn` prints to stdout
- `stubInspectImage(t, info, err)` — replaces `inspectImage` for the test; returns a restore func

Follow CLAUDE.md AAA pattern: `// Arrange`, `// Act`, `// Assert` with blank lines between sections.

### 4. Update `bin/agentic`

a. **Remove** `usage_<cmd>()` — help is now served by `agentic-cli <cmd> --help`

b. **Remove** `cmd_<cmd>()` — logic lives in Go

c. **Replace** the dispatch case:

```bash
<cmd>)
  exec agentic-cli <cmd> "${@:2}"
  ;;
```

d. **Replace** the `cmd_help` case:

```bash
<cmd>) exec agentic-cli <cmd> --help ;;
```

### 5. Clean up shell scripts

Check whether any bash helper scripts are now dead code:

- **`tools/<cmd>/` script** — if the command had a `tools/<name>/<cmd>.sh` per tool (e.g. `clean.sh`), delete them now that Go handles the logic directly.
- **`shared/scripts/<cmd>-common.sh`** — delete if it was only sourced by the per-tool scripts above.
- **`tools/*/config.sh` comments** — update the header comment (e.g. `# This file is sourced by build.sh, clean.sh and update.sh`) to remove the deleted script.
- **`CLAUDE.md` tool structure** — remove the deleted script from the `tools/<name>/` required-files list and from the `shared/scripts/` description.

### 6. Verify

```bash
go test ./cmd/...

# build and smoke-test (adjust install path as needed)
go build -o ~/.local/bin/agentic-cli .

agentic <cmd>
agentic help <cmd>
agentic <cmd> --help
```
