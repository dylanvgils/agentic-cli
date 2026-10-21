package sandbox

import "slices"

// Proxy modes; ProxyOff is the zero value, so a RunSpec without one runs no proxy.
const (
	ProxyOff ProxyMode = iota
	ProxyEnforce
	ProxyMonitor
)

// ProxyMode selects whether the egress proxy runs and whether it blocks or only logs disallowed hosts.
type ProxyMode int

// AllowRule allows Host on every path, or with Paths only on those, which the proxy decrypts the host's traffic to
// check; Host is exact or a "."/"*." wildcard, exact when it has Paths.
type AllowRule struct {
	Host  string
	Paths []string
}

// ProxySpec configures the egress proxy sidecar; unless Mode is ProxyOff, the tool reaches out only through it.
type ProxySpec struct {
	Mode    ProxyMode
	Rules   []AllowRule // merged allowlist: tool baseline, user hosts and path rules, credential hosts
	LogFile string      // host path of this run's JSON-lines access log
}

// Enabled reports whether the proxy runs, in either mode.
func (m ProxyMode) Enabled() bool {
	return m != ProxyOff
}

// HasPathRules reports whether any rule allows only some paths.
func (s ProxySpec) HasPathRules() bool {
	return slices.ContainsFunc(s.Rules, func(r AllowRule) bool { return len(r.Paths) > 0 })
}

// WholeHosts returns the hosts of the rules that allow every path.
func (s ProxySpec) WholeHosts() []string {
	var hosts []string
	for _, rule := range s.Rules {
		if len(rule.Paths) == 0 {
			hosts = append(hosts, rule.Host)
		}
	}
	return hosts
}
