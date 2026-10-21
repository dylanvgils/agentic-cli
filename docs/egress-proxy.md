# Egress proxy

The egress proxy limits which hosts a tool can reach and logs every attempt. It can also hold API keys for the tool, so the agent never sees them: see [Credential injection](credential-injection.md).

## Usage

```bash
agentic run --proxy claude           # block hosts not on the allowlist
agentic run --proxy-monitor claude   # log only, block nothing
agentic run --no-proxy claude        # turn off a proxy enabled in config
```

Or enable it per project - see [`[run.proxy]`](config.md#runproxy) for the keys:

```toml
[run.proxy]
enabled = true
allowed_hosts = ["registry.npmjs.org", ".github.com"]
```

## Baseline allowlist

Each tool allows these hosts by default. `allowed_hosts` adds to them, and [path rules](#path-rules) allow a host only on some paths.

| Tool       | Baseline host        | Purpose                             |
| ---------- | -------------------- | ----------------------------------- |
| `claude`   | `.anthropic.com`     | Claude API and telemetry subdomains |
| `claude`   | `.claude.ai`         | installer and asset downloads       |
| `claude`   | `.claude.com`        | OAuth/login flow and docs           |
| `copilot`  | `.githubcopilot.com` | Copilot API and subdomains          |
| `copilot`  | `api.github.com`     | GitHub API used for authentication  |
| `copilot`  | `docs.github.com`    | Copilot CLI docs                    |
| `opencode` | `opencode.ai`        | OpenCode auth, update checks, docs  |

OpenCode supports many model providers, so add your provider's hosts:

```toml
[run.proxy]
allowed_hosts = ["api.openai.com"]
```

## Path rules

`[[run.proxy.allowed_paths]]` allows a host only on some paths, e.g. one repo on `raw.githubusercontent.com` instead of every file on it:

```toml
[[run.proxy.allowed_paths]]
host = "raw.githubusercontent.com"
paths = [
  "/anthropics/claude-code/*/*/main/CHANGELOG.md", # refs/heads/main
  "/dylanvgils/agentic-cli/**",
]
```

A pattern matches the whole path, not the query:

- A literal segment matches exactly.
- `*` matches one segment.
- A final `**` matches everything below, including nothing.

How the proxy applies them:

- **Decrypted**: the proxy decrypts the host's HTTPS to read each path, with the same CA trust as [credential hosts](credential-injection.md#how-it-works). Tools that pin certificates fail on it.
- **Refused**: a path no pattern matches gets a 403, logged as `path-blocked` and listed at the end of the run. So does a path with `%2F`, `%5C`, `%2E`, `%3B`, `;`, `\`, or a `.` or `..` segment, which an upstream may read as another path.
- **Path rules win**: they limit the host even when `allowed_hosts`, a tool's baseline or a wildcard allows it whole. Other hosts the wildcard matches stay whole.
- **Credential hosts** only get their secret on the allowed paths.
- **Baseline hosts**: a rule for a host the tool needs, like `api.anthropic.com`, limits the tool too.

## Monitor mode

`--proxy-monitor` (or `mode = "monitor"`) never blocks. Use it to find what a tool needs before you enforce:

1. Run with `--proxy-monitor` and use the tool as usual.
2. At the end, the run lists the hosts and paths that _would_ have been blocked.
3. Add the ones you trust to `allowed_hosts` or `allowed_paths`.
4. Switch to `--proxy`.

The log still records the real `"decision"` (`"allow"` or `"deny"`), tagged `"enforced": false`, and `docker logs` lines get a `(monitor)` suffix.

## How it works

- The tool container loses direct internet access. It reaches the outside only through a proxy sidecar on a per-run internal Docker network.
- `HTTP_PROXY`/`HTTPS_PROXY` (and lowercase variants) are set automatically.
- The proxy image builds on first use, or with `agentic proxy build`. `agentic proxy update` rebuilds it.
- The sidecar and its network are removed when the run ends. After a crash, the next `agentic` run or `agentic clean` removes them once they are a few minutes old and the tool has stopped.
- `agentic config` shows the resolved proxy settings and which file set them, except the fixed [baseline hosts](#baseline-allowlist).

### Access log

Blocked hosts are printed at the end of the run. Every attempt is logged as JSON lines in `$AGENTIC_HOME/logs/proxy_<id>.jsonl`, pruned after `proxy_log_retention_days` in `agentic.json` (default 3). `agentic proxy clean --logs` wipes them all.

Failed requests get a `"reason"`, with the cause in `"error"`:

| Reason | Tool sees | When |
| --- | --- | --- |
| `upstream-failed` | 502 | The upstream host fails. `"error"` can name resolved IPs the tool shouldn't see. |
| `proxy-failed` | 500 | The proxy itself fails, e.g. on a TLS handshake the tool rejects. |
| `client-closed` | - | The tool hangs up before or during the TLS handshake of a tunnel, or cancels a request to an intercepted host before the response. Harmless when the tool races connections or drops requests it no longer needs. |
| `path-blocked` | 403 | No [path rule](#path-rules) of the host allows the request's `"path"`. |
| `sni-mismatch` | Closed tunnel | The tunnel's TLS server name isn't the host it asked for, or the tunnel isn't TLS (e.g. SSH through `CONNECT`). Plain HTTP works through `HTTP_PROXY`. |

## Local and private addresses

The proxy checks the IP an allowed host resolves to, so a DNS record can't point it at your machine or network. This also applies in monitor mode, where hosts not on the allowlist get the wildcard rules. A refused attempt gets a 403, is logged as `"decision": "deny"` with `"reason": "blocked-address"`, and is listed apart from blocked hosts at the end of the run.

- **Always refused**: loopback, link-local (incl. the `169.254.169.254` cloud metadata address), unspecified, multicast and reserved (`240/4`) addresses, Teredo, SIIT and local-use NAT64 addresses, and the AWS, Azure and Alibaba metadata endpoints outside link-local.
- **Refused for wildcard entries**: private ranges (`10/8`, `172.16/12`, `192.168/16`, `100.64/10`, `198.18/15`, `fc00::/7`). An exact entry like `"artifactory.corp.example"` may resolve to one, so list internal hosts exactly.
- **NAT64 and 6to4 addresses** are checked by the IPv4 address inside them.
- **With credential injection**, the tunnel is logged as allowed first, then each refused request gets its own deny entry.

## Tools that ignore proxy env vars

Some tools, like Maven, ignore those env vars and need a literal `host:port` in their own config. Use `agentic-proxy:3128`. Unlike the sidecar's container name, this alias is the same on every run. See [Maven through the egress proxy](recipes.md#maven-through-the-egress-proxy).

## With microsandbox

With the [microsandbox runtime](microsandbox.md), msb's own network policy enforces the same allowlist instead of a proxy sidecar, so no proxy env vars are set. Only DNS and the allowed hosts on ports 80 and 443 are reachable. msb inspects all HTTPS to check each request's host, so the tool must trust msb's certificate authority (curl, git and Bun do), and tools that pin certificates fail.

- There's no access log and no summary at the end of the run.
- `--proxy-monitor` is refused; use `--proxy`.
- An internal host that resolves to a private address isn't reachable, since msb drops private DNS answers.
- Every `allowed_hosts` entry must be a hostname like `api.example.test` or `.example.test`, else the run is refused. msb would read a CIDR or a name like `public` as a wider rule.
- [Credentials](credential-injection.md) keep working, since msb injects them itself rather than a proxy.
- [Path rules](#path-rules) are refused; use `--runtime docker`.
