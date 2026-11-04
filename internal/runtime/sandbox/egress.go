package sandbox

import (
	"fmt"
	"slices"
	"strings"
)

// Egress modes; EgressOff is the zero value, so a RunSpec without one leaves egress unfiltered.
const (
	EgressOff EgressMode = iota
	EgressEnforce
	EgressMonitor
)

// egressIntegrations are the tools a run can route through egress control though they ignore the proxy env vars, sorted.
var egressIntegrations = []string{"gradle", "maven"}

// EgressMode selects whether egress is filtered and whether a disallowed host is blocked or only logged.
type EgressMode int

// AllowRule allows Host on every path, or with Paths only on those, which the runtime decrypts the host's traffic to
// check; Host is exact or a "."/"*." wildcard, exact when it has Paths.
type AllowRule struct {
	Host  string
	Paths []string
}

// EgressPolicy is what the tool may reach; the runtime decides how to enforce it. Unless Mode is EgressOff, the tool
// reaches only what Rules allow.
type EgressPolicy struct {
	Mode  EgressMode
	Rules []AllowRule // merged allowlist: tool baseline, user hosts and path rules, credential hosts
	// Integrations name tools to route through egress control, e.g. "maven"
	Integrations []string
}

// Enabled reports whether egress is filtered, in either mode.
func (m EgressMode) Enabled() bool {
	return m != EgressOff
}

// HasPathRules reports whether any rule allows only some paths.
func (p EgressPolicy) HasPathRules() bool {
	return slices.ContainsFunc(p.Rules, func(r AllowRule) bool { return len(r.Paths) > 0 })
}

// WholeHosts returns the hosts of the rules that allow every path.
func (p EgressPolicy) WholeHosts() []string {
	var hosts []string
	for _, rule := range p.Rules {
		if len(rule.Paths) == 0 {
			hosts = append(hosts, rule.Host)
		}
	}
	return hosts
}

// AllowWhole returns rules with each one allowing every path, for a runtime that can't match paths.
func AllowWhole(rules []AllowRule) []AllowRule {
	whole := make([]AllowRule, len(rules))
	for i, rule := range rules {
		whole[i] = AllowRule{Host: rule.Host}
	}
	return whole
}

// EgressIntegrations returns the known egress integration names, sorted.
func EgressIntegrations() []string {
	return slices.Clone(egressIntegrations)
}

// CheckEgressIntegrations refuses the first name that isn't a known egress integration.
func CheckEgressIntegrations(names []string) error {
	for _, name := range names {
		if !slices.Contains(egressIntegrations, name) {
			return fmt.Errorf("unknown integration %q: must be one of %s", name, strings.Join(egressIntegrations, ", "))
		}
	}
	return nil
}
