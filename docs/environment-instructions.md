# Environment instructions

Each run writes a block into the tool's own global instructions file that tells the model about its environment, so it doesn't waste turns on things that fail. The block says the project's own `CLAUDE.md`/`AGENTS.md` wins on conflicts. Configure it with [`[run.instructions]`](config.md#runinstructions).

| Tool        | File                                 |
| ----------- | ------------------------------------ |
| Claude Code | `~/.claude/CLAUDE.md`                |
| OpenCode    | `~/.config/opencode/AGENTS.md`       |
| Copilot CLI | `~/.copilot/copilot-instructions.md` |

## What it covers

- **What's installed**: the base toolchain, runtimes, apt packages and custom installs, read from the built image. The base toolchain is listed even before the first build.
- **What's restricted**: writable paths, resource limits and privileges. In a container that's a read-only root and dropped capabilities; with the [microsandbox runtime](microsandbox.md), a writable root that's discarded when the VM exits.
- **Network**, with `--proxy`: no direct internet access, the allowlist, and a note to tell you about blocked hosts so you can allow them.
- **Docker**, with `--dind`: how to reach the daemon and its ports, which paths it can bind mount, and its limits.
- **Your own text** from `custom`, appended last.

## Preview

```bash
agentic instructions claude
agentic instructions claude --proxy --dind --runtime microsandbox
```

It takes the same `--proxy`, `--dind`, `--runtime` and limit flags as `agentic run`, so you see the block that run would write.

## How the file is handled

The rest of the file is left alone, including notes the tool saves there itself, even with `enabled = false`. Each run works on a private copy of the file, so concurrent runs don't affect each other. Changes made during the run are copied back when the tool exits.

The file must be a regular file: agentic refuses a symlink there, since the tool can write that directory and could otherwise redirect the copy-back to any host file. Reads and writes also stay inside `$AGENTIC_HOME/tools`.
