package run

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// printProxySummary reports what the run's proxy log flagged, if anything, set apart from the tool output above it.
func printProxySummary(l *logging.Logger, spec sandbox.ProxySpec) {
	s := proxy.ReadSummary(spec.LogFile)
	if !s.Flagged() {
		return
	}

	fmt.Fprintln(l.Writer())
	printDenied(l, s, spec.Mode == sandbox.ProxyMonitor)
	printPaths(l, s.Paths, spec.Mode == sandbox.ProxyMonitor)
	printRefused(l, s.Refused)
	printMismatched(l, s.Mismatched, spec.Mode == sandbox.ProxyMonitor)
	printMissed(l, s.Missed)
}

// printDenied reports hosts the allowlist blocked, or would have blocked in monitor mode.
func printDenied(l *logging.Logger, s proxy.Summary, monitor bool) {
	if s.Denied.Count == 0 {
		return
	}

	hosts := strings.Join(s.Denied.Hosts, ", ")
	if monitor {
		l.Infof("proxy (monitor mode) observed %d request(s); %d would be blocked under the current allowlist: %s", s.Attempts, s.Denied.Count, hosts)
		l.Detail("add the ones you want to allow to [run.proxy] allowed_hosts, then drop --proxy-monitor.")
		return
	}

	l.Infof("proxy blocked %d request(s) to: %s", s.Denied.Count, hosts)
	l.Detail("add them to [run.proxy] allowed_hosts (or pass --no-proxy) to permit.")
}

// printPaths reports paths the path rules blocked, or would have blocked in monitor mode.
func printPaths(l *logging.Logger, paths proxy.HostCount, monitor bool) {
	if paths.Count == 0 {
		return
	}

	verb := "blocked"
	if monitor {
		verb = "would block"
	}
	l.Infof("proxy %s %d request(s) to paths no rule allows: %s", verb, paths.Count, strings.Join(paths.Hosts, ", "))
	l.Detail("add the ones you want to allow to their host's paths in [[run.proxy.allowed_paths]].")
}

// printRefused reports hosts the address guard refused, which it does in both modes.
func printRefused(l *logging.Logger, refused proxy.HostCount) {
	if refused.Count == 0 {
		return
	}

	l.Infof("proxy refused %d request(s) to local or private addresses: %s", refused.Count, strings.Join(refused.Hosts, ", "))
	l.Detail("list an internal host exactly in [run.proxy] allowed_hosts to permit its private address; loopback and link-local stay refused.")
}

// printMismatched warns about tunnels whose TLS server name wasn't the host they asked for, refused unless monitor.
func printMismatched(l *logging.Logger, mismatched proxy.HostCount, monitor bool) {
	if mismatched.Count == 0 {
		return
	}

	verb := "refused"
	if monitor {
		verb = "saw"
	}
	l.Warnf("proxy %s %d tunnel(s) whose TLS server name wasn't the host they connected to: %s", verb, mismatched.Count, strings.Join(mismatched.Hosts, ", "))
	l.Detail("the tool may be reaching a site that isn't allowlisted through an allowed host; tunnels must carry TLS for that host.")
}

// printMissed warns about credential hosts the tool reached without the placeholder.
func printMissed(l *logging.Logger, missed proxy.HostCount) {
	if missed.Count == 0 {
		return
	}

	l.Warnf("proxy saw %d connection(s) to credential hosts without the placeholder, so they likely ran unauthenticated: %s", missed.Count, strings.Join(missed.Hosts, ", "))
	l.Detail("check that the tool reads one of the credential's env names and sends it in a listed header.")
}
