package sandbox

import "github.com/dylanvgils/agentic-cli/internal/proxy"

// Proxy modes; ProxyOff is the zero value, so a RunSpec without one runs no proxy.
const (
	ProxyOff ProxyMode = iota
	ProxyEnforce
	ProxyMonitor
)

// ProxyMode selects whether the egress proxy runs and whether it blocks or only logs disallowed hosts.
type ProxyMode int

// ProxySpec configures the egress proxy sidecar; unless Mode is ProxyOff, the tool reaches out only through it.
type ProxySpec struct {
	Mode   ProxyMode
	Image  string   // proxy sidecar image
	Allow  []string // merged allowlist (tool baseline + user hosts)
	LogDir string   // host dir for JSON-lines access logs
	// Credentials are the headers the proxy injects; they hold secrets, so never print them
	Credentials []proxy.Credential
}

// Enabled reports whether the proxy runs, in either mode.
func (m ProxyMode) Enabled() bool {
	return m != ProxyOff
}
