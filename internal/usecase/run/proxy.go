package run

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
)

// defaultProxyLogRetentionDays is how long proxy access logs are kept when no retention period is configured.
const defaultProxyLogRetentionDays = 3

// hostTally counts one kind of access-log entry and keeps their unique hosts in first-seen order, made safe to print.
type hostTally struct {
	hosts []string
	count int
	seen  map[string]bool
}

// proxyLogSummary is what a run's access log shows: entries the allowlist denied, the address guard refused and
// credential tunnels without the placeholder, plus the connection attempts.
type proxyLogSummary struct {
	denied   hostTally
	refused  hostTally
	missed   hostTally
	attempts int
}

// add counts an entry for host.
func (t *hostTally) add(host string) {
	t.count++
	if t.seen == nil {
		t.seen = make(map[string]bool)
	}
	if !t.seen[host] {
		t.seen[host] = true
		t.hosts = append(t.hosts, proxy.DisplayHost(host))
	}
}

// proxyLogFile returns a new access-log path in the tool home's logs dir when the proxy is enabled, else an empty string.
func proxyLogFile(toolHome string, proxyEnabled bool) (string, error) {
	if !proxyEnabled {
		return "", nil
	}

	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("name proxy log: %w", err)
	}
	return filepath.Join(toolHome, config.LogsDirName, proxy.LogFilePrefix+hex.EncodeToString(buf)+".jsonl"), nil
}

// prepareProxyLogDir creates dir and prunes the proxy logs in it older than toolHome's retention window.
func prepareProxyLogDir(dir, toolHome string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	proxy.PruneLogs(dir, time.Duration(proxyRetentionDays(toolHome))*24*time.Hour)
	return nil
}

// proxyRetentionDays resolves the proxy log retention window from agentic.json (a host-level setting, not .agenticrc.toml/a flag), falling back to the default when unset.
func proxyRetentionDays(toolHome string) int {
	if cfg, err := config.LoadConfig(toolHome); err == nil && cfg.ProxyLogRetentionDays > 0 {
		return cfg.ProxyLogRetentionDays
	}
	return defaultProxyLogRetentionDays
}

// printProxySummary reports hosts the allowlist blocked, or would have blocked in monitor mode, separately hosts the
// address guard refused, which it does in both modes, and credential hosts the tool reached without the placeholder.
func printProxySummary(l *logging.Logger, spec sandbox.ProxySpec) {
	s := readProxyLog(spec.LogFile)
	if s.denied.count == 0 && s.refused.count == 0 && s.missed.count == 0 {
		return
	}

	// Blank line separates the summary from the tool output above it
	fmt.Fprintln(l.Writer())

	switch {
	case s.denied.count == 0:
	case spec.Mode == sandbox.ProxyMonitor:
		l.Infof("proxy (monitor mode) observed %d request(s); %d would be blocked under the current allowlist: %s", s.attempts, s.denied.count, strings.Join(s.denied.hosts, ", "))
		l.Detail("add the ones you want to allow to [run.proxy] allowed_hosts, then drop --proxy-monitor.")
	default:
		l.Infof("proxy blocked %d request(s) to: %s", s.denied.count, strings.Join(s.denied.hosts, ", "))
		l.Detail("add them to [run.proxy] allowed_hosts (or pass --no-proxy) to permit.")
	}

	if s.refused.count > 0 {
		l.Infof("proxy refused %d request(s) to local or private addresses: %s", s.refused.count, strings.Join(s.refused.hosts, ", "))
		l.Detail("list an internal host exactly in [run.proxy] allowed_hosts to permit its private address; loopback and link-local stay refused.")
	}

	if s.missed.count > 0 {
		l.Warnf("proxy saw %d connection(s) to credential hosts without the placeholder, so they likely ran unauthenticated: %s", s.missed.count, strings.Join(s.missed.hosts, ", "))
		l.Detail("check that the tool reads one of the credential's env names and sends it in a listed header.")
	}
}

// readProxyLog sums up the access log at path in one pass, skipping lines it can't parse; an unreadable log sums to
// nothing.
func readProxyLog(path string) proxyLogSummary {
	var s proxyLogSummary
	f, err := os.Open(path)
	if err != nil {
		return s
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry proxy.Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		if !repeatsAttempt(entry) {
			s.attempts++
		}
		switch {
		case deniedByAllowlist(entry):
			s.denied.add(entry.Host)
		case refusedByGuard(entry):
			s.refused.add(entry.Host)
		case missedPlaceholder(entry):
			s.missed.add(entry.Host)
		}
	}

	// A read error only means the summary covers what was scanned before it; there's no error path to report it
	_ = scanner.Err()

	return s
}

// deniedByAllowlist reports whether entry was denied because its host isn't on the allowlist, even if monitor mode then
// let it through to an upstream that failed.
func deniedByAllowlist(entry proxy.Entry) bool {
	return entry.Decision == proxy.DecisionDeny && entry.Reason != proxy.ReasonBlockedAddr && entry.Reason != proxy.ReasonNoPlaceholder
}

// missedPlaceholder reports whether entry is a credential tunnel whose requests never carried the placeholder.
func missedPlaceholder(entry proxy.Entry) bool {
	return entry.Reason == proxy.ReasonNoPlaceholder
}

// repeatsAttempt reports whether entry was logged from inside an intercepted tunnel, such as a failed request or a
// missed placeholder, after the tunnel's own entry.
func repeatsAttempt(entry proxy.Entry) bool {
	return entry.Intercepted && entry.Reason != ""
}

// refusedByGuard reports whether the address guard refused entry.
func refusedByGuard(entry proxy.Entry) bool {
	return entry.Reason == proxy.ReasonBlockedAddr
}
