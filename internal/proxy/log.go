package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Decision records whether a connection attempt was permitted.
type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
)

// ReasonBlockedAddr is the log reason for an allowed host refused by the address guard.
const ReasonBlockedAddr = "blocked-address"

// ReasonUpstreamFailed is the log reason for an allowed request the upstream couldn't serve, with the detail in Entry.Error.
const ReasonUpstreamFailed = "upstream-failed"

// ReasonProxyFailed is the log reason for an allowed connection the proxy couldn't set up itself, such as a TLS handshake
// the client rejected, with the detail in Entry.Error.
const ReasonProxyFailed = "proxy-failed"

// ReasonNoPlaceholder is the log reason for a closed credential tunnel the upstream rejected (401/403) and whose requests
// never carried the placeholder. It repeats the tunnel's entry rather than counting a new attempt.
const ReasonNoPlaceholder = "no-placeholder"

// Protocol records how the client reached the proxy: an HTTP CONNECT tunnel (HTTPS) or a plain HTTP forward.
type Protocol string

const (
	ProtocolHTTP  Protocol = "http"
	ProtocolHTTPS Protocol = "https"
)

// LogFilePrefix distinguishes proxy access-log filenames from other log types in $TOOL_HOME/logs.
const LogFilePrefix = "proxy_"

// Entry is a single structured access-log record, emitted as one JSON line per connection attempt.
type Entry struct {
	Time     time.Time `json:"time"`
	Protocol Protocol  `json:"protocol"`
	Host     string    `json:"host"`
	Port     string    `json:"port"`
	Decision Decision  `json:"decision"`
	// Enforced reports whether Decision was acted on; always false in monitor mode, where a "deny" is only observed, not blocked.
	Enforced bool `json:"enforced"`
	// Intercepted reports whether the tunnel was TLS-terminated for a credential host; not whether a secret was swapped in.
	Intercepted bool `json:"intercepted,omitempty"`
	// Reason says why an allowed host was still refused or failed, e.g. ReasonBlockedAddr.
	Reason string `json:"reason,omitempty"`
	// Error is the upstream or proxy failure detail, kept out of the response to the tool; never in the human line.
	Error string `json:"error,omitempty"`
}

// Logger writes each access record as a JSON line (always UTC) to an optional file and as a
// human-readable line (shown in location, typically stdout for `docker logs -f`) to an optional
// destination. Safe for concurrent use.
type Logger struct {
	mutex    sync.Mutex
	encoder  *json.Encoder // nil when no JSON destination is configured
	human    io.Writer     // nil when no human-readable destination is configured
	location *time.Location
	now      func() time.Time
}

// NewLogger returns a Logger writing JSON to file and human-readable lines (in location, nil defaulting to UTC) to human; either writer may be nil.
func NewLogger(file, human io.Writer, location *time.Location) *Logger {
	if location == nil {
		location = time.UTC
	}

	l := &Logger{human: human, location: location, now: time.Now}
	if file != nil {
		l.encoder = json.NewEncoder(file)
	}
	return l
}

// Log records a single connection attempt, stamping entry.Time with the current time.
func (l *Logger) Log(entry Entry) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	entry.Time = l.now().UTC()

	if l.encoder != nil {
		_ = l.encoder.Encode(entry)
	}

	if l.human != nil {
		level := "[" + strings.ToUpper(string(entry.Decision)) + "]"
		tags := ""
		if !entry.Enforced {
			tags += " (monitor)"
		}
		if entry.Intercepted {
			tags += " (intercepted)"
		}
		if entry.Reason != "" {
			tags += " (" + entry.Reason + ")"
		}
		fmt.Fprintf(l.human, "%s %-7s %-5s %s:%s%s\n", entry.Time.In(l.location).Format(time.RFC3339), level, entry.Protocol, DisplayHost(entry.Host), entry.Port, tags)
	}
}

// DisplayHost returns host for a terminal, quoted when it has anything but printable ASCII, such as bidi overrides the tool
// could use to spoof the line.
func DisplayHost(host string) string {
	for _, c := range host {
		if c <= ' ' || c > '~' {
			return strconv.QuoteToASCII(host)
		}
	}
	return host
}

// PruneLogs removes access-log files in dir older than maxAge (maxAge <= 0 removes all). Best-effort: never fails the caller.
func PruneLogs(dir string, maxAge time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), LogFilePrefix) || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}

		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
}
