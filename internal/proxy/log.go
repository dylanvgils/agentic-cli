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

// Protocol records how the client reached the proxy: an HTTP CONNECT tunnel (HTTPS) or a plain HTTP forward.
type Protocol string

const (
	ProtocolHTTP  Protocol = "http"
	ProtocolHTTPS Protocol = "https"
)

// Entry is a single structured access-log record, emitted as one JSON line per connection attempt.
type Entry struct {
	Time     time.Time `json:"time"`
	Protocol Protocol  `json:"protocol"`
	Host     string    `json:"host"`
	Port     string    `json:"port"`
	Decision Decision  `json:"decision"`
	// Enforced reports whether Decision was acted on; always false in monitor mode, where a "deny" is only observed, not blocked.
	Enforced bool `json:"enforced"`
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

// Log records a single connection attempt; enforced is Entry.Enforced - pass false from monitor mode.
func (l *Logger) Log(protocol Protocol, host, port string, decision Decision, enforced bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	entry := Entry{
		Time:     l.now().UTC(),
		Protocol: protocol,
		Host:     host,
		Port:     port,
		Decision: decision,
		Enforced: enforced,
	}

	if l.encoder != nil {
		_ = l.encoder.Encode(entry)
	}

	if l.human != nil {
		level := "[" + strings.ToUpper(string(decision)) + "]"
		tag := ""
		if !enforced {
			tag = " (monitor)"
		}
		fmt.Fprintf(l.human, "%s %-7s %-5s %s:%s%s\n", entry.Time.In(l.location).Format(time.RFC3339), level, protocol, host, port, tag)
	}
}
