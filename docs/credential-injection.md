# Credential injection

`[[run.proxy.credentials]]` keeps API keys out of the tool container. Inside it, each entry's env vars hold a random placeholder, new each run, which the tool sends like a real key. The [egress proxy](egress-proxy.md) holds the real secret and swaps it in for the placeholder on HTTPS requests to the entry's hosts. See [`[[run.proxy.credentials]]`](config.md#runproxycredentials) for the keys and presets.

## Usage

1. Save the key in a one-line file outside your projects, e.g. `~/.secrets/anthropic_key`.
2. Add an entry to `.agenticrc.toml`:

   ```toml
   [[run.proxy.credentials]]
   preset = "anthropic"
   secret = "~/.secrets/anthropic_key"
   ```

3. Run the tool and approve the entry when asked.
4. If an older image fails, run `agentic update <tool>`.

With the [microsandbox runtime](microsandbox.md), msb swaps the secret in itself, without the proxy, and git over HTTPS sends no credentials.

## How it works

- **Proxy on**: credentials turn the proxy on in enforce mode (monitor mode still applies if set), and their hosts join the allowlist, limited to their paths when [`allowed_paths`](egress-proxy.md#path-rules) names them. Combining them with `--no-proxy` or `enabled = false` is an error.
- **Only credential hosts are decrypted**: the proxy terminates TLS only for those hosts and [path-limited hosts](egress-proxy.md#path-rules), using a per-run CA that can sign nothing else. All other traffic stays an end-to-end encrypted tunnel. Plain HTTP is never touched. Terminated hosts use HTTP/1.1, and their log entries carry `"intercepted": true`.
- **Only the placeholder is swapped**: in the entry's headers, decoding `Basic` auth first, and nowhere else. A request without the placeholder passes unchanged. Look-alike headers such as `X_Api_Key` for `X-Api-Key` are dropped.
- **Missing placeholder warning**: if an entry's host rejects a connection (401 or 403) that never sent the placeholder, the run ends with a warning, as the tool likely ran unauthenticated. Anonymous requests the host accepts, such as cloning a public repo, don't warn. The log marks it `"reason": "no-placeholder"`.
- **git**: with the `github` preset, agentic sets a git credential helper through `GIT_CONFIG_*` env vars. git over HTTPS then answers GitHub's challenge with `x-access-token` and the placeholder.
- **Secrets stay on the host**: agentic reads each secret when the run starts and copies it into the sidecar, which is removed when the run ends. A secret file inside the current directory, or inside any path mounted into the container, is refused.
- **Approval**: entries need your approval when they first appear and whenever they change, like other [guarded settings](config.md#setting-approval). Each run prints `agentic: injecting credentials for <hosts> (<secret path>)`.
- **CA trust**: the entrypoint adds the CA to the system bundle and points `SSL_CERT_FILE`, `CURL_CA_BUNDLE`, `GIT_SSL_CAINFO`, `REQUESTS_CA_BUNDLE` and `NODE_EXTRA_CA_CERTS` at it. This covers OpenSSL, Go, curl, git, Python and Node/Bun. These vars, `AGENTIC_PROXY_CA`, the entries' `env` names and, with the `github` preset, the `GIT_CONFIG_*` vars can't be set with `--env`.

## Limitations

- **Static keys only**: OAuth logins (Claude subscription, Copilot device flow) keep their token in the tool home. Setting a key's env var (e.g. `ANTHROPIC_API_KEY`) switches the tool to API-key auth.
- **Copilot**: the `github` preset also covers the Copilot API hosts, including the remote GitHub MCP server. GHE.com data-residency tenants need a custom entry for their own Copilot hosts.
- **Headers only**: a tool that sends the key in the URL or body, or signs requests with it (e.g. AWS SigV4), can't use a placeholder.
- **Use, not theft**: the agent can't read the key, but it can still make authenticated requests to the key's hosts during the run. `allowed_paths` can limit that to some paths, though a path like GitHub's `/graphql` reaches everything, so a token the provider scopes stays the stronger limit.
- **Echoed keys**: the proxy swaps the key back to the placeholder in response headers and trailers, also in the Basic-encoded form git sends, but not in bodies. A credential host that echoes the request in its body hands the key to the tool.
- **Gaps in CA trust**: `agentic run <tool> -- <cmd>` skips the entrypoint, and Java and containers started through `--dind` use their own trust stores. TLS to credential and path-limited hosts fails in those cases.
