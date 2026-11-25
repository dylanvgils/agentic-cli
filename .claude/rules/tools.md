---
paths:
  - "internal/image/tools/**"
---
# Tools and Dockerfile DSL rules

- New tool:
  - A `Configs` entry in `internal/image/tools/tools.go`.
  - `internal/image/tools/<name>.go`, with `claude` as the template.
  - Reuse `createContainerUser`, `aptInstallRun`, `sandbox.VolumeMount`.
- New runtime layer:
  - A case in `extraStage()` and names in `knownExtras`/`LayerFlagDesc` (`bases.go`).
  - A `Versions` field with defaults in `ForLayer`/`versions.json`.
  - Any base-stage apt packages in `layerPackages`.
  - No `--<name>` flag; it registers itself.
- DSL:
  - Install steps use `df.Run{Blocks: ...}`, version check scripts `df.Heredoc`.
  - One `df.Block` per logical phase; independent commands get their own block.
  - A multi-line command is several `Lines` in one block; `Chain: true` for `&&`-chained lines.
  - Pipelines need `pipefailShell` first.
  - Multi-line entrypoint or heredoc shell goes in `scripts/*.sh`, rendered by `renderScript`/`scriptBlock` with `scriptVars` (consts as `{{.X}}`, `${ {{- .X}}:-}` for a default); one-liners stay Go strings.
