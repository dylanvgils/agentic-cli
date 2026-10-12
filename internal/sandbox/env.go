package sandbox

import (
	"os"
	"strings"
)

// terminalEnvNames are host vars forwarded into the sandbox; not reserved, so an env entry can override one.
var terminalEnvNames = []string{"COLORTERM", "TERM", "NO_COLOR", "FORCE_COLOR"}

// ResolveEnv returns the sandbox env as KEY=VALUE: forwarded terminal vars and host timezone, then
// env entries ("KEY=VALUE", or bare "KEY" to forward the host's value); later entries win.
func ResolveEnv(env []string) []string {
	var resolved []string
	for _, key := range terminalEnvNames {
		if val := os.Getenv(key); val != "" {
			resolved = append(resolved, key+"="+val)
		}
	}

	if tz := hostTimezone(); tz != "" {
		resolved = append(resolved, "TZ="+tz)
	}

	for _, entry := range env {
		if key, _, ok := strings.Cut(entry, "="); ok {
			resolved = append(resolved, entry)
		} else if val, set := os.LookupEnv(key); set {
			resolved = append(resolved, key+"="+val)
		}
	}

	return resolved
}
