package sandbox

import (
	"os"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/platform"
)

// ProxyCAEnvName carries the proxy CA cert PEM into the tool, which agentic sets when proxy credentials are configured.
const ProxyCAEnvName = "AGENTIC_PROXY_CA"

// configEnvNames are env vars agentic sets for every run.
var configEnvNames = []string{
	"TOOL_HOME",            // the tool's state dir in the container
	"CONTAINER_HOME",       // the container user's home
	"AGENTIC_MARKETPLACES", // names of the mounted plugin marketplaces
	ProxyCAEnvName,
}

// proxyEnvNames point the tool at the egress proxy; overriding one would silently bypass the allowlist. Both cases, as
// tools read either.
var proxyEnvNames = []string{
	"HTTP_PROXY",
	"HTTPS_PROXY",
	"http_proxy",
	"https_proxy",
	"NO_PROXY",
	"no_proxy",
}

// dindEnvNames point the tool's docker CLI at the sidecar over mutual TLS; overriding one would bypass it.
var dindEnvNames = []string{
	"DOCKER_HOST",
	"DOCKER_TLS_VERIFY",
	"DOCKER_CERT_PATH",
	"DOCKER_CONTEXT", // a context would pick another daemon
	"DOCKER_CONFIG",  // a config could set a context or credential helpers
}

// terminalEnvNames are host vars forwarded into the sandbox; not reserved, so an env entry can override one.
var terminalEnvNames = []string{"COLORTERM", "TERM", "NO_COLOR", "FORCE_COLOR"}

// IsReservedEnvName reports whether key is an env var agentic manages in a run with or without the proxy and dind, so an
// env entry can't override it.
func IsReservedEnvName(key string, proxy, dind bool) bool {
	return slices.Contains(configEnvNames, key) ||
		(proxy && slices.Contains(proxyEnvNames, key)) ||
		(dind && slices.Contains(dindEnvNames, key))
}

// ResolveEnv returns the sandbox env as KEY=VALUE: forwarded terminal vars and host timezone, then env entries; later entries win.
func ResolveEnv(env []string) []string {
	return slices.Concat(terminalEnv(), timezoneEnv(platform.Timezone()), entryEnv(env))
}

// EnvArgs returns ResolveEnv(env) as --env flags, which both runtimes take.
func EnvArgs(env []string) []string {
	var args []string
	for _, entry := range ResolveEnv(env) {
		args = append(args, Arg("env", entry))
	}
	return args
}

// terminalEnv returns the terminal vars set on the host.
func terminalEnv() []string {
	var resolved []string
	for _, key := range terminalEnvNames {
		if val := os.Getenv(key); val != "" {
			resolved = append(resolved, key+"="+val)
		}
	}
	return resolved
}

// timezoneEnv returns TZ for the host timezone tz, or nothing when it couldn't be detected.
func timezoneEnv(tz string) []string {
	if tz != "" {
		return []string{"TZ=" + tz}
	}
	return nil
}

// entryEnv resolves env entries: "KEY=VALUE" as is, bare "KEY" to the host's value, dropped when unset.
func entryEnv(env []string) []string {
	var resolved []string
	for _, entry := range env {
		if key, _, ok := strings.Cut(entry, "="); ok {
			resolved = append(resolved, entry)
		} else if val, set := os.LookupEnv(key); set {
			resolved = append(resolved, key+"="+val)
		}
	}
	return resolved
}
