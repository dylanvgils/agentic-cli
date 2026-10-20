package sandbox

// Proxy modes; ProxyOff is the zero value, so a RunSpec without one runs no proxy.
const (
	ProxyOff ProxyMode = iota
	ProxyEnforce
	ProxyMonitor
)

// ProxyMode selects whether the egress proxy runs and whether it blocks or only logs disallowed hosts.
type ProxyMode int

// AllowRule allows Host, exact or a "."/"*." wildcard.
type AllowRule struct {
	Host string
}

// ProxySpec configures the egress proxy sidecar; unless Mode is ProxyOff, the tool reaches out only through it.
type ProxySpec struct {
	Mode    ProxyMode
	Rules   []AllowRule // merged allowlist: tool baseline, user hosts, credential hosts
	LogFile string      // host path of this run's JSON-lines access log
}

// Enabled reports whether the proxy runs, in either mode.
func (m ProxyMode) Enabled() bool {
	return m != ProxyOff
}

// Hosts returns the host of every rule.
func (s ProxySpec) Hosts() []string {
	hosts := make([]string, len(s.Rules))
	for i, rule := range s.Rules {
		hosts[i] = rule.Host
	}
	return hosts
}
