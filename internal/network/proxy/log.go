package proxy

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LogFilePrefix distinguishes proxy access-log filenames from other log types in $TOOL_HOME/logs.
const LogFilePrefix = "proxy_"

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
)

const (
	ProtocolHTTP  Protocol = "http"
	ProtocolHTTPS Protocol = "https"
)

// Log reasons say why an allowed host was still refused or failed.
const (
	// ReasonBlockedAddr is an allowed host refused by the address guard.
	ReasonBlockedAddr Reason = "blocked-address"

	// ReasonUpstreamFailed is an allowed request the upstream couldn't serve, with the detail in Entry.Error.
	ReasonUpstreamFailed Reason = "upstream-failed"

	// ReasonProxyFailed is an allowed connection the proxy couldn't set up itself, such as a TLS handshake the client
	// rejected, with the detail in Entry.Error.
	ReasonProxyFailed Reason = "proxy-failed"

	// ReasonClientClosed is an allowed tunnel the client closed before or during the TLS handshake, or an intercepted
	// request the client cancelled before the response, with the detail in Entry.Error.
	ReasonClientClosed Reason = "client-closed"

	// ReasonSNIMismatch is a tunnel to an allowed host whose TLS server name (SNI) names another host, or that doesn't
	// start with TLS, with the name or error in Entry.Error.
	ReasonSNIMismatch Reason = "sni-mismatch"

	// ReasonPathBlocked is a request to a path rule host for a path none of its rules allows, with the path in Entry.Path,
	// or a tunnel to one the proxy couldn't decrypt to check.
	ReasonPathBlocked Reason = "path-blocked"

	// ReasonNoPlaceholder is a closed credential tunnel the upstream rejected (401/403) and whose requests never carried
	// the placeholder. It repeats the tunnel's entry rather than counting a new attempt.
	ReasonNoPlaceholder Reason = "no-placeholder"
)

// Decision records whether a connection attempt was permitted.
type Decision string

// Reason records why an allowed host was still refused or failed.
type Reason string

// Protocol records how the client reached the proxy: an HTTP CONNECT tunnel (HTTPS) or a plain HTTP forward.
type Protocol string

// Entry is a single structured access-log record, emitted as one JSON line per connection attempt.
type Entry struct {
	Time     time.Time `json:"time"`
	Protocol Protocol  `json:"protocol"`
	Host     string    `json:"host"`
	Port     string    `json:"port"`
	Decision Decision  `json:"decision"`
	// Enforced reports whether Decision was acted on; always false in monitor mode, where a "deny" is only observed, not blocked.
	Enforced bool `json:"enforced"`
	// Intercepted reports whether the tunnel was TLS-terminated for a credential or path rule host; not whether a secret
	// was swapped in.
	Intercepted bool `json:"intercepted,omitempty"`
	// Path is the escaped request path a path rule refused, without the query, which can carry tokens.
	Path string `json:"path,omitempty"`
	// Reason says why an allowed host was still refused or failed, e.g. ReasonBlockedAddr.
	Reason Reason `json:"reason,omitempty"`
	// Error is the upstream or proxy failure detail, kept out of the response to the tool; never in the human line.
	Error string `json:"error,omitempty"`
}

// Logger writes each entry as a UTC JSON line to an optional file and as a slog "request" record. Concurrency-safe.
type Logger struct {
	mutex   sync.Mutex
	encoder *json.Encoder // nil when no JSON destination is configured
	now     func() time.Time
}

// refused returns e denied for reason, enforced unless in monitor mode.
func (e Entry) refused(reason Reason, monitor bool) Entry {
	e.Decision = DecisionDeny
	e.Enforced = !monitor
	e.Reason = reason
	return e
}

// NewLogHandler returns the handler for everything the proxy logs: key=value text lines on w, with times in location.
func NewLogHandler(w io.Writer, location *time.Location) slog.Handler {
	return slog.NewTextHandler(w, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				a.Value = slog.TimeValue(a.Value.Time().In(location))
			}
			return a
		},
	})
}

// NewLogger returns a Logger writing JSON to file (nil for none).
func NewLogger(file io.Writer) *Logger {
	l := &Logger{now: time.Now}
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

	_ = slog.Default().Handler().Handle(context.Background(), requestRecord(entry))
}

// DisplayHost quotes host for a terminal when it isn't printable ASCII, so bidi tricks can't spoof the line.
func DisplayHost(host string) string {
	for _, c := range host {
		if c <= ' ' || c > '~' {
			return strconv.QuoteToASCII(host)
		}
	}
	return host
}

// PruneLogs removes access logs in dir older than maxAge (<= 0 removes all), ignoring errors.
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

// requestRecord returns entry as an info "request" record stamped with its time, its flags only when set.
func requestRecord(entry Entry) slog.Record {
	r := slog.NewRecord(entry.Time, slog.LevelInfo, "request", 0)
	r.AddAttrs(
		slog.String("proto", string(entry.Protocol)),
		slog.String("host", entry.Host),
		slog.String("port", entry.Port),
	)
	if entry.Path != "" {
		r.AddAttrs(slog.String("path", entry.Path))
	}
	r.AddAttrs(slog.String("decision", string(entry.Decision)))
	if !entry.Enforced {
		r.AddAttrs(slog.Bool("monitor", true))
	}
	if entry.Intercepted {
		r.AddAttrs(slog.Bool("intercepted", true))
	}
	if entry.Reason != "" {
		r.AddAttrs(slog.String("reason", string(entry.Reason)))
	}
	return r
}

// httpErrorLog returns a std logger whose lines become slog errors, for net/http's ErrorLog.
func httpErrorLog() *log.Logger {
	return slog.NewLogLogger(slog.Default().Handler(), slog.LevelError)
}
