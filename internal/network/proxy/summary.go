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

// Summary counts a run's access log by kind of refusal, plus the connection attempts.
type Summary struct {
	Denied     HostCount
	Paths      HostCount
	Refused    HostCount
	Mismatched HostCount
	Missed     HostCount
	Attempts   int
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
	return s.Denied.Count > 0 || s.Paths.Count > 0 || s.Refused.Count > 0 || s.Mismatched.Count > 0 || s.Missed.Count > 0
}

// add counts entry as an attempt unless it repeats one, and under its refusal kind.
func (s *Summary) add(entry Entry) {
	if !entry.repeatsAttempt() {
		s.Attempts++
	}

	switch entry.Reason {
	case ReasonPathBlocked:
		s.Paths.add(entry.Host + entry.Path)
	case ReasonBlockedAddr:
		s.Refused.add(entry.Host)
	case ReasonSNIMismatch:
		s.Mismatched.add(entry.Host)
	case ReasonNoPlaceholder:
		s.Missed.add(entry.Host)
	default:
		if entry.Decision == DecisionDeny {
			s.Denied.add(entry.Host)
		}
	}
}

// repeatsAttempt reports whether e follows its tunnel's own entry, or is a monitor mode path verdict.
func (e Entry) repeatsAttempt() bool {
	return (e.Intercepted && e.Reason != "") || (e.Reason == ReasonPathBlocked && !e.Enforced)
}

// ReadSummary sums up the access log at path, skipping unparsable lines; an unreadable log sums to nothing.
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
