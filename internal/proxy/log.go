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

// Protocol records how the client reached the proxy: an HTTP CONNECT tunnel
// (used for HTTPS) or a plain HTTP forward.
type Protocol string

const (
	ProtocolHTTP  Protocol = "http"
	ProtocolHTTPS Protocol = "https"
)

// Entry is a single structured access-log record, emitted as one JSON line per
// connection attempt. The shape is intentionally stable so a future insights UI
// can consume the log directly.
type Entry struct {
	Time     time.Time `json:"time"`
	Protocol Protocol  `json:"protocol"`
	Host     string    `json:"host"`
	Port     string    `json:"port"`
	Decision Decision  `json:"decision"`
}

// Logger writes each access record as a JSON line to an optional file and as a
// human-readable line to an optional human-readable destination (typically
// stdout, so `docker logs -f` on the proxy container is easy to read). The
// JSON line always records UTC; the human-readable line is shown in location,
// since it's meant to be read live by whoever is watching the container's
// logs. It is safe for concurrent use: each connection is handled in its own
// goroutine.
type Logger struct {
	mutex    sync.Mutex
	encoder  *json.Encoder // nil when no JSON destination is configured
	human    io.Writer     // nil when no human-readable destination is configured
	location *time.Location
	now      func() time.Time
}

// NewLogger returns a Logger that writes JSON lines to file and human-readable
// lines to human, with the human-readable line shown in location (nil
// defaults to UTC). Either writer may be nil to skip that destination.
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

// Log records a single connection attempt.
func (l *Logger) Log(protocol Protocol, host, port string, decision Decision) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	entry := Entry{Time: l.now().UTC(), Protocol: protocol, Host: host, Port: port, Decision: decision}

	if l.encoder != nil {
		_ = l.encoder.Encode(entry)
	}

	if l.human != nil {
		level := "[" + strings.ToUpper(string(decision)) + "]"
		fmt.Fprintf(l.human, "%s %-7s %-5s %s:%s\n", entry.Time.In(l.location).Format(time.RFC3339), level, protocol, host, port)
	}
}
