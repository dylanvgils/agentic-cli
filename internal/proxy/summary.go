package proxy

import (
	"bufio"
	"encoding/json"
	"os"
)

// HostCount counts one kind of access-log entry and keeps their unique hosts in first-seen order, made safe to print.
type HostCount struct {
	Hosts []string
	Count int
	seen  map[string]bool
}

// Summary is what a run's access log shows: entries the allowlist denied, the address guard refused and credential
// tunnels without the placeholder, plus the connection attempts.
type Summary struct {
	Denied   HostCount
	Refused  HostCount
	Missed   HostCount
	Attempts int
}

// add counts an entry for host.
func (c *HostCount) add(host string) {
	c.Count++
	if c.seen[host] {
		return
	}

	if c.seen == nil {
		c.seen = make(map[string]bool)
	}
	c.seen[host] = true
	c.Hosts = append(c.Hosts, DisplayHost(host))
}

// Flagged reports whether the log shows anything the user should hear about.
func (s Summary) Flagged() bool {
	return s.Denied.Count > 0 || s.Refused.Count > 0 || s.Missed.Count > 0
}

// add counts entry as an attempt, unless it repeats one, and under the one kind it belongs to.
func (s *Summary) add(entry Entry) {
	if !entry.repeatsAttempt() {
		s.Attempts++
	}

	switch {
	case entry.deniedByAllowlist():
		s.Denied.add(entry.Host)
	case entry.refusedByGuard():
		s.Refused.add(entry.Host)
	case entry.missedPlaceholder():
		s.Missed.add(entry.Host)
	}
}

// deniedByAllowlist reports whether e was denied because its host isn't on the allowlist, even if monitor mode then let
// it through to an upstream that failed.
func (e Entry) deniedByAllowlist() bool {
	return e.Decision == DecisionDeny && e.Reason != ReasonBlockedAddr && e.Reason != ReasonNoPlaceholder
}

// refusedByGuard reports whether the address guard refused e.
func (e Entry) refusedByGuard() bool {
	return e.Reason == ReasonBlockedAddr
}

// missedPlaceholder reports whether e is a credential tunnel whose requests never carried the placeholder.
func (e Entry) missedPlaceholder() bool {
	return e.Reason == ReasonNoPlaceholder
}

// repeatsAttempt reports whether e was logged from inside an intercepted tunnel, such as a failed request or a missed
// placeholder, after the tunnel's own entry.
func (e Entry) repeatsAttempt() bool {
	return e.Intercepted && e.Reason != ""
}

// ReadSummary sums up the access log at path in one pass, skipping lines it can't parse; an unreadable log sums to
// nothing.
func ReadSummary(path string) Summary {
	var s Summary
	f, err := os.Open(path)
	if err != nil {
		return s
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err == nil {
			s.add(entry)
		}
	}

	// A read error only means the summary covers what was scanned before it
	_ = scanner.Err()
	return s
}
