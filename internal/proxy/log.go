package proxy

import (
	"encoding/json"
	"fmt"
	"io"
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
	// Injected reports whether the tunnel was TLS-terminated to inject credentials.
	Injected bool `json:"injected,omitempty"`
	// Reason says why an allowed host was still refused, e.g. ReasonBlockedAddr.
	Reason string `json:"reason,omitempty"`
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
		if entry.Injected {
			tags += " (injected)"
		}
		if entry.Reason != "" {
			tags += " (" + entry.Reason + ")"
		}
		fmt.Fprintf(l.human, "%s %-7s %-5s %s:%s%s\n", entry.Time.In(l.location).Format(time.RFC3339), level, entry.Protocol, entry.Host, entry.Port, tags)
	}
}
