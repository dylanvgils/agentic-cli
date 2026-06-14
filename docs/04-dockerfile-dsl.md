# Dockerfile DSL

`internal/dockerfile` provides a typed Go DSL for generating Dockerfiles programmatically. Static Dockerfile files do not exist in this project, every image is built from a `dockerfile.File` assembled at build time.

The DSL avoids string templates and raw concatenation: instructions are typed structs, stages are composed values, and every rendered instruction carries a comment pointing back to the Go file and line that produced it.

## Type system

Everything implements `Instruction`:

```go
type Instruction interface {
    Render() string
}
```

### Instructions

`instructions.go` defines the standard directives:

| Type         | Fields           | Renders as                          |
| ------------ | ---------------- | ----------------------------------- |
| `From`       | `Image`, `As`    | `FROM image AS name`                |
| `Arg`        | `Key`, `Default` | `ARG key=default`                   |
| `Env`        | `Key`, `Value`   | `ENV key=value`                     |
| `Shell`      | `Cmd []string`   | `SHELL ["a", "b"]` (exec form)      |
| `User`       | `Name`           | `USER name`                         |
| `Workdir`    | `Path`           | `WORKDIR /path`                     |
| `Label`      | `Key`, `Value`   | `LABEL key=value`                   |
| `Entrypoint` | `Cmd []string`   | `ENTRYPOINT ["a", "b"]` (exec form) |

### Run

`Run` (`run.go`) has three modes - use the one that fits:

```go
// Single pre-formatted string
Run{Command: "npm ci --omit=dev"}

// Flat sequence, joined with backslash continuation
Run{Lines: []string{"apt-get install -y", "curl", "git"}}

// Grouped blocks, && joined; each block can carry an optional shell comment
Run{Blocks: []Block{
    {Lines: []string{"apt-get update -yq"}},
    {Lines: []string{"apt-get install -yq --no-install-recommends", "curl", "wget"}},
    {Lines: []string{"rm -rf /var/lib/apt/lists/*"}},
}}
```

The `Blocks` form renders as:

```dockerfile
RUN apt-get update -yq \
  && apt-get install -yq --no-install-recommends \
  curl \
  wget \
  && rm -rf /var/lib/apt/lists/*
```

Add a `Comment` to a block to insert a shell comment before its lines:

```go
{Comment: "Create container user", Lines: []string{"useradd -m claude"}}
```

### Heredoc

`Heredoc` (`heredoc.go`) writes a multi-line script using BuildKit's `COPY --chmod=0755 <<'EOF'` syntax. The `--chmod` flag sets the executable bit at copy time, so no separate `RUN chmod +x` is needed and the instruction works correctly regardless of the active `USER` context:

```go
Heredoc{
    Dest:  "/usr/local/bin/entrypoint.sh",
    Lines: []string{"#!/usr/bin/env bash", "set -euo pipefail", `exec claude "$@"`},
}
```

Renders as:

```dockerfile
COPY --chmod=0755 <<'EOF' /usr/local/bin/entrypoint.sh
#!/usr/bin/env bash
set -euo pipefail
exec claude "$@"
EOF
```

### Located

`Located` (`located.go`) wraps any `Instruction` with Dockerfile comments. `StageBuilder.Add()` creates one automatically for each instruction, capturing the Go `file:line` of the call site, so the rendered Dockerfile always shows where each instruction came from.

To attach a human-readable annotation on top of the source location, wrap the instruction with `C()` before passing it to `Add()`:

```go
Add(df.C("Create container user", df.Run{Lines: []string{"useradd -m claude"}}))
```

Renders as:

```dockerfile
# Create container user
# internal/tools/claude.go:41
RUN useradd -m claude
```

## Stage and File

`Stage` holds one FROM block; `File` collects stages into a complete Dockerfile:

```go
type Stage struct {
    GlobalArgs   []Arg         // ARG directives placed before FROM, used in the image spec
    From         From
    Instructions []Instruction
}

type File struct {
    Stages []Stage
}
```

`File.Render()` emits each stage with a `######` section divider and the stage name, global args before FROM, then instructions separated by blank lines.

## Builder API

`StageBuilder` is the fluent interface for constructing a `Stage`. Every `Add()` call wraps the instruction(s) in `Located` with the calling Go source location automatically.

`Add()` is variadic - pass one instruction or several in a single call:

```go
// Single instruction
Add(df.Env{Key: "DEBIAN_FRONTEND", Value: "noninteractive"})

// Multiple instructions in one call (share the same source location)
Add(df.Arg{Key: "HOST_UID", Default: "1000"}, df.Arg{Key: "HOST_GID", Default: "1000"})

// Spread a slice returned by a helper
Add(CreateContainerUser("claude")...)
```

Full example:

```go
stage := df.NewStage(df.From{Image: "debian:${DEBIAN_VERSION}", As: "base"}).
    AddGlobalArg(df.Arg{Key: "DEBIAN_VERSION", Default: "bookworm-slim"}).
    Add(df.Env{Key: "DEBIAN_FRONTEND", Value: "noninteractive"}).
    Add(df.Run{Blocks: []df.Block{
        {Lines: []string{"apt-get update -yq"}},
        {Lines: []string{"apt-get install -yq --no-install-recommends", "curl", "wget", "git"}},
        {Lines: []string{"rm -rf /var/lib/apt/lists/*"}},
    }}).
    Build()
```

Partial rendered output:

```dockerfile
##############################
# base
##############################
ARG DEBIAN_VERSION=bookworm-slim

FROM debian:${DEBIAN_VERSION} AS base

# internal/tools/bases.go:23
ENV DEBIAN_FRONTEND=noninteractive

# internal/tools/bases.go:28
RUN apt-get update -yq \
  && apt-get install -yq --no-install-recommends \
  curl \
  wget \
  git \
  && rm -rf /var/lib/apt/lists/*
```

## Multi-stage composition

Each tool and extra stage is a function that receives `prevStage string` and returns a `df.Stage` that builds FROM it:

```go
func claudeStage(prevStage string) df.Stage {
    return df.NewStage(df.From{Image: prevStage, As: "tool"}).
        // ...
        Build()
}
```

`internal/tools/generate.go` threads these together - base → extras → tool stage - then calls `File{Stages: stages}.Render()` to produce the final Dockerfile string. The `Stage` field in each tool's `BuildConfig` (`internal/tools/tools.go`) is what gets wired into this pipeline.
