package run

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
)

// printProxySummary reports what the run's proxy log flagged, if anything, set apart from the tool output above it.
func printProxySummary(l *logging.Logger, spec sandbox.ProxySpec) {
	s := proxy.ReadSummary(spec.LogFile)
	if !s.Flagged() {
		return
	}

	fmt.Fprintln(l.Writer())
	printDenied(l, s, spec.Mode == sandbox.ProxyMonitor)
	printRefused(l, s.Refused)
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

// printRefused reports hosts the address guard refused, which it does in both modes.
func printRefused(l *logging.Logger, refused proxy.HostCount) {
	if refused.Count == 0 {
		return
	}

	l.Infof("proxy refused %d request(s) to local or private addresses: %s", refused.Count, strings.Join(refused.Hosts, ", "))
	l.Detail("list an internal host exactly in [run.proxy] allowed_hosts to permit its private address; loopback and link-local stay refused.")
}

// printMissed warns about credential hosts the tool reached without the placeholder.
func printMissed(l *logging.Logger, missed proxy.HostCount) {
	if missed.Count == 0 {
		return
	}

	l.Warnf("proxy saw %d connection(s) to credential hosts without the placeholder, so they likely ran unauthenticated: %s", missed.Count, strings.Join(missed.Hosts, ", "))
	l.Detail("check that the tool reads one of the credential's env names and sends it in a listed header.")
}
