# Images

Build the image(s) you need:

```bash
agentic build                        # Build all tools
agentic build claude                 # Claude agent only
agentic build copilot                # GitHub Copilot agent only
agentic build opencode               # OpenCode agent only
agentic build claude --base node,java # Claude with Node.js and Java runtimes added
agentic build claude --no-cache      # Force a fully fresh build, re-pulling base images
```

To remove all containers and images created by this project:

```bash
agentic clean
```

## Base images

Debian is the root layer. The `--base` flag adds extra runtimes on top of it, including Node.js (installed via NVM):

```
debian (base stage)
  ├── docker (docker stage) ← added with --base docker (CLI only, for --dind)
  ├── dotnet (dotnet stage) ← added with --base dotnet
  ├── go     (go stage)     ← added with --base go
  ├── java   (java stage)   ← added with --base java
  └── node   (node stage)   ← added with --base node
        └── tool (tool stage)
```

All stages are composed into a single multi-stage Dockerfile at build time and built in one `docker build` call. No intermediate images are produced.

| Flag                                   | Result                         |
| -------------------------------------- | ------------------------------ |
| _(none)_                               | debian only                    |
| `--base node`                          | debian + Node.js               |
| `--base node,java`                     | debian + Node.js + Java        |
| `--base node,java,dotnet`              | debian + Node.js + Java + .NET |
| `--node 22`                            | debian + Node.js v22           |
| `--base node,java --java 17`           | debian + Node.js + Java 17     |
| `--node 22 --base node,java --java 17` | debian + Node.js v22 + Java 17 |

Use `--base` to add extra runtimes at build time. The same pinning pattern applies to every layer (`--debian`, `--node`, `--dotnet`, `--go`, `--docker`, etc.).

`--base` merges with `.agenticrc.toml`'s `bases` setting (see [Configuration](config.md)) - it can only add to the configured list, never remove from it. Use `--base-exact` instead to replace the resolved list outright, ignoring `.agenticrc.toml`'s `bases` entirely - `--base-exact node` builds with only Node.js regardless of what's configured, and `--base-exact=` (empty) builds debian only. `--base` and `--base-exact` are mutually exclusive.

## Versions and updates

Version defaults are embedded in the binary at build time - run `agentic build --help` to see current defaults. Override per-build with the corresponding flag (`--debian`, `--node`, `--java`, `--dotnet`, `--go`), or pin a persistent default per project with `[build.versions]` in `.agenticrc.toml` (see [Configuration](config.md)).

`agentic update` reuses the version each layer was originally built with, so base/extra layers are regenerated identically (and stay cache-hits) even if the embedded defaults have since changed - pass the flag again to pin a different version instead. Pass `--no-cache` to also rebuild the base/extra layers from scratch, instead of only the tool stage.

Base images (`debian:13-slim`, `golang:1.26.6`, etc.) use floating tags, not pinned digests, so registries publish security patches under the same tag over time. `agentic update` defaults to `--pull`, checking for a newer base image at most once every 24h per image (tracked via an `agentic.pulled` image label) - even when the tool's own CLI version is current - so patches reach your images without requiring `--no-cache`. Pass `--pull` to force a check now, or `--pull=false` to disable it (e.g. offline). `agentic build` does not pull by default - pass `--pull` explicitly to fetch fresh base images at build time too. `agentic update` prints the installed and predicted target version (`version: X -> Y`, or `version: X (up to date)`) before the rebuild starts, not just after.

> **Note:** During `agentic update`, the `bases` and `apt_packages` settings from `.agenticrc.toml` are ignored - the original build configuration is always reused. Only an explicit `--base`/`--base-exact` or `--apt`/`--apt-exact` CLI flag overrides what the image was built with - `--base-exact`/`--apt-exact` win over both `.agenticrc.toml` and the previously-built image's recovered state, even when passed as an empty list.

`agentic run` also checks upstream for a newer tool version automatically (at most once every 6 hours per tool) and, in an interactive terminal, prompts you to update before the tool starts - the same upstream check `agentic update` does, just run proactively. Answering "no" (or running non-interactively) just prints a one-line notice and starts the tool on the current version. Disable it per project with `check_updates = false` under `[run]` in `.agenticrc.toml` (see [Configuration](config.md)).

## Extra apt packages

Use `--apt` to install additional Debian packages into the base stage, verified against `apt-cache show` before the build starts (fail-fast):

```bash
agentic build claude --apt make
agentic build claude --apt make,gcc   # comma-separated or repeatable (--apt make --apt gcc)
```

`agentic update` automatically reuses the package list, so you don't need to re-specify it each time. For persisting packages via `.agenticrc.toml`, and for registry proxy configuration (pulling base images through Harbor, Nexus, Artifactory, etc.), see [Configuration](config.md).

`--apt` merges with `.agenticrc.toml`'s `apt_packages` setting - it can only add packages, never remove one that's configured. Use `--apt-exact` to replace the resolved list outright, ignoring `.agenticrc.toml`'s `apt_packages` entirely (`--apt-exact make,gcc`), or `--apt-exact=` (empty) to install no extra packages at all. `--apt` and `--apt-exact` are mutually exclusive.

Use `agentic inspect` to see base layers, apt packages, build timestamp, and installed tool version for any built image.

## Custom installs

For tools not packaged via apt (e.g. `helm`, `golangci-lint`), declare a `[[build.custom_installs]]` entry in `.agenticrc.toml`:

```toml
[[build.custom_installs]]
name = "helm"
run = [
  "curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 -o /tmp/get-helm.sh",
  "bash /tmp/get-helm.sh",
]
```

Runs as root before the tool's own install step - install into a root-owned path like `/usr/local/bin` rather than `$HOME`. See [Configuration](config.md) for the full reference.
