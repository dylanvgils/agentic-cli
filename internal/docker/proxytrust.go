package docker

import (
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// dryRunCAPlaceholder stands in for the CA cert in dry-run output, since the CA is only issued at start.
const dryRunCAPlaceholder = "<proxy CA issued at start>"

// trustArgs passes the proxy CA cert to the tool, whose entrypoint adds it to a CA bundle; nil when the proxy doesn't decrypt.
func (h proxyHandle) trustArgs() []string {
	if !h.decrypts() {
		return nil
	}

	caPEM := string(h.caPEM)
	if caPEM == "" {
		caPEM = dryRunCAPlaceholder
	}
	return []string{arg("env", tools.ProxyCAEnvName+"="+caPEM)}
}
