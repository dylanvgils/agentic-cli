package sandbox

import (
	"os"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/platform"
)

// terminalEnvNames are host vars forwarded into the sandbox; not reserved, so an env entry can override one.
var terminalEnvNames = []string{"COLORTERM", "TERM", "NO_COLOR", "FORCE_COLOR"}

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
