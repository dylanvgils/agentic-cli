# Images

Each tool runs from an image built on your machine. `agentic build` builds it, `agentic update` rebuilds it with the latest tool version, and `agentic clean` removes it.

```bash
agentic build                         # all tools
agentic build claude --base node,java # add Node.js and Java
agentic build claude --no-cache       # fully fresh build
agentic build claude --pull           # refresh base images, keep the rest of the cache
```

`agentic inspect <tool>` shows an image's layers, apt packages, custom installs, build time and tool version.

## Runtime layers

Debian is the root layer. `--base` adds runtimes on top of it (Node.js is installed with NVM):

```
debian (base stage)
  ├── docker (docker stage) ← --base docker (CLI only, for --dind)
  ├── dotnet (dotnet stage) ← --base dotnet
  ├── go     (go stage)     ← --base go
  ├── java   (java stage)   ← --base java
  └── node   (node stage)   ← --base node
        └── tool (tool stage)
```

All stages go into one multi-stage Dockerfile and are built in a single `docker build`, with no intermediate images.

| Flag                         | Result                         |
| ---------------------------- | ------------------------------ |
| _(none)_                     | debian only                    |
| `--base node,java`           | debian + Node.js + Java        |
| `--base node,java,dotnet`    | debian + Node.js + Java + .NET |
| `--base node,java --java 17` | debian + Node.js + Java 17     |
| `--node 22`                  | debian + Node.js v22           |

`--base` adds to the `bases` in `.agenticrc.toml`. `--base-exact` replaces them (see [Precedence](config.md#precedence)).

## Versions and updates

- Default versions are built into the binary (see `agentic build --help`). Pin one per build with `--debian`, `--node`, `--java`, `--dotnet`, `--go` or `--docker`, or per project with [`[build.versions]`](config.md#build).
- `agentic update` reuses the runtimes, versions and apt packages the image was built with, and ignores `bases`/`apt_packages` in `.agenticrc.toml`. Only an explicit `--base`, `--apt`, `--<layer>` or `--*-exact` flag changes them. Unchanged layers stay cached. `--no-cache` rebuilds them anyway.
- `agentic update` prints `version: X -> Y` (or `X (up to date)`) before it starts.
- Base images use floating tags, so registries publish patches under the same tag. `agentic update` pulls them at most once every 24h per image, even when the tool is current. `--pull` forces a check and `--pull=false` turns it off. `agentic build` only pulls with `--pull`.
- `agentic run` checks for a newer tool version at most every 6 hours, and in a terminal offers to update first. Saying no, or a non-interactive run, prints a notice and starts the current version. Turn it off with `check_updates = false` under `[run]`.

## Extra apt packages

`--apt` installs extra Debian packages. They're checked with `apt-cache show` before the build starts.

```bash
agentic build claude --apt make,gcc   # or repeat: --apt make --apt gcc
```

`--apt` adds to `apt_packages` in `.agenticrc.toml`, and `--apt-exact` replaces them (see [Precedence](config.md#precedence)).

## Custom installs

For tools that aren't packaged for apt (e.g. `helm`), see [`[[build.custom_installs]]`](config.md#buildcustom_installs).
