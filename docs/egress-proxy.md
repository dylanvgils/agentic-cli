# Egress proxy

> [!WARNING]
> **Beta** - the egress proxy and credential injection are under active development. Config keys and behavior may change between releases, and they haven't had the same testing as the core container hardening. Issue reports are welcome.

The egress proxy limits which hosts a tool can reach and logs every attempt. It can also hold API keys for the tool, so the agent never sees them.

## Usage

```bash
agentic claude --proxy           # block hosts not on the allowlist
agentic claude --proxy-monitor   # log only, block nothing
agentic claude --no-proxy        # turn off a proxy enabled in config
```

Or enable it per project - see [`[run.proxy]`](config.md#runproxy) for the keys:

```toml
[run.proxy]
enabled = true
allowed_hosts = ["registry.npmjs.org", ".github.com"]
```

## How it works

- The tool container loses direct internet access. It reaches the outside only through a proxy sidecar on a per-run internal Docker network.
- `HTTP_PROXY`/`HTTPS_PROXY` (and lowercase variants) are set automatically.
- Blocked hosts are printed at the end of the run. Every attempt is logged as JSON lines in `$AGENTIC_HOME/logs/proxy_<id>.jsonl`.
- Logs older than `proxy_log_retention_days` in `agentic.json` (default 3) are pruned on each proxy run. `agentic proxy clean --logs` wipes them all.
- The sidecar and its network are removed when the run ends. After a crash, the next `agentic` run removes them once they are a few minutes old, and `agentic clean` removes them right away.
- The proxy image builds on first use, or with `agentic proxy build`. `agentic proxy update` rebuilds it.
- `agentic config` shows the resolved proxy settings and which file set them. The baseline hosts below aren't listed, because they can't be changed.

### Monitor mode

`--proxy-monitor` (or `mode = "monitor"`) never blocks. The log still records the real `"decision"` (`"allow"` or `"deny"`), tagged `"enforced": false`, and `docker logs` lines get a `(monitor)` suffix. The run ends with the hosts that _would_ have been blocked. Use it to find a tool's egress needs before you switch to enforcement.

### Baseline allowlist

Each tool allows these hosts by default. `allowed_hosts` adds to them.

| Tool       | Baseline host        | Purpose                             |
| ---------- | -------------------- | ----------------------------------- |
| `claude`   | `.anthropic.com`     | Claude API and telemetry subdomains |
| `claude`   | `.claude.ai`         | installer and asset downloads       |
| `claude`   | `.claude.com`        | OAuth/login flow                    |
| `copilot`  | `.githubcopilot.com` | Copilot API and subdomains          |
| `copilot`  | `api.github.com`     | GitHub API used for authentication  |
| `opencode` | `opencode.ai`        | OpenCode auth and update checks     |

OpenCode supports many model providers, so add your provider's hosts to `allowed_hosts`.

### Tools that ignore proxy env vars

Some tools, like Maven, ignore those env vars and need a literal `host:port` in their own config. Use `agentic-proxy:3128`. Unlike the sidecar's container name, this alias is the same on every run. See [Maven through the egress proxy](recipes.md#maven-through-the-egress-proxy).

## Credential injection

`[[run.proxy.credentials]]` keeps API keys out of the tool container. The proxy holds the real secret and sets it as a header on HTTPS requests to the entry's hosts. Inside the container, the entry's env vars hold only the placeholder `agentic-proxy-managed`, so tools that need a key still start. See [`[[run.proxy.credentials]]`](config.md#runproxycredentials) for the keys and presets.

- **Proxy on**: credentials turn the proxy on in enforce mode (monitor mode still applies if set), and their hosts join the allowlist. Combining them with `--no-proxy` or `enabled = false` is an error.
- **Only credential hosts are decrypted**: the proxy terminates TLS only for those hosts, using a per-run CA that can sign nothing else. It always overwrites the header. All other traffic stays an end-to-end encrypted tunnel. Plain HTTP is never injected. Terminated hosts use HTTP/1.1, and their log entries carry `"injected": true`.
- **Secrets stay on the host**: agentic reads each secret when the run starts and copies it into the sidecar, which is removed when the run ends. A secret file inside the current directory, or inside any path mounted into the container, is refused.
- **Approval**: you approve each entry when it first appears and again whenever it changes, because an agent can edit config files. The prompt lists each entry's preset or hosts and its secret path. Non-interactive runs fail until the entry is approved. Each run prints `agentic: injecting credentials for <hosts> (<secret path>)`.
- **CA trust**: the entrypoint adds the CA to the system bundle and points `SSL_CERT_FILE`, `CURL_CA_BUNDLE`, `GIT_SSL_CAINFO`, `REQUESTS_CA_BUNDLE` and `NODE_EXTRA_CA_CERTS` at it. This covers OpenSSL, Go, curl, git, Python and Node/Bun. These vars, `AGENTIC_PROXY_CA` and the entries' `env` names can't be set with `--env`.
- **Older images** fail with a hint to run `agentic update <tool>`.

### Limitations

- **Static keys only**: OAuth logins (Claude subscription, Copilot device flow) keep their token in the tool home. Setting a key's env var (e.g. `ANTHROPIC_API_KEY`) switches the tool to API-key auth.
- **Copilot**: the `github` preset injects the GitHub token, but Copilot exchanges it for a short-lived token that comes back in a response body, where the agent can read it.
- **Use, not theft**: the agent can't read the key, but it can still make authenticated requests to the key's hosts during the run.
- **Gaps in CA trust**: `agentic run <tool> -- <cmd>` skips the entrypoint, and Java and containers started through `--dind` use their own trust stores. TLS to credential hosts fails in those cases.
