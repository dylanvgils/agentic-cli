package fswatch

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Op identifies the kind of filesystem activity an Entry records.
type Op string

const (
	OpOpen   Op = "open"   // file was opened - a read or write intent
	OpWrite  Op = "write"  // a writable file descriptor was closed
	OpCreate Op = "create" // file or directory created
	OpDelete Op = "delete" // file or directory removed
	OpRename Op = "rename" // file or directory moved into or out of a watched directory
)

// Entry is one JSON-line audit record; a Detail-only entry (Op/Path unset) records meta info.
type Entry struct {
	Time   time.Time `json:"time"`
	Op     Op        `json:"op,omitempty"`
	Path   string    `json:"path,omitempty"`
	IsDir  bool      `json:"is_dir,omitempty"`
	Detail string    `json:"detail,omitempty"`
}

// Logger writes each Entry as a JSON line to w. Safe for concurrent use.
type Logger struct {
	mu  sync.Mutex
	enc *json.Encoder
	now func() time.Time
}

// NewLogger returns a Logger writing JSON lines to w.
func NewLogger(w io.Writer) *Logger {
	return &Logger{enc: json.NewEncoder(w), now: time.Now}
}

// Log records a single filesystem event for path.
func (l *Logger) Log(op Op, path string, isDir bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_ = l.enc.Encode(Entry{Time: l.now().UTC(), Op: op, Path: path, IsDir: isDir})
}

// LogDetail records a meta-entry not tied to a specific filesystem event.
func (l *Logger) LogDetail(detail string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_ = l.enc.Encode(Entry{Time: l.now().UTC(), Detail: detail})
}
