// Package cmdlog records the commands agentic runs, such as docker and msb calls, as JSON lines in a log file.
package cmdlog

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/infra/hostfs"
)

// Entry is one command, written as one JSON line like the proxy's access log.
type Entry struct {
	Time    time.Time `json:"time"`
	Command string    `json:"command"`
	// Args must hold no secrets; the caller masks them.
	Args []string `json:"args"`
	// ExitCode is the command's exit code, or -1 when it didn't exit, with the cause in Error.
	ExitCode       int    `json:"exit_code"`
	Error          string `json:"error,omitempty"`
	DurationMillis int64  `json:"duration_ms"`
}

// Log records commands to the file Open opened, and nothing while closed. Safe for concurrent use.
type Log struct {
	mu      sync.Mutex
	file    io.Closer
	encoder *json.Encoder
}

// New returns a Log that records nothing until Open.
func New() *Log {
	return &Log{}
}

// Open appends every later command to path, creating it private to this user; an open file is closed first.
func (l *Log) Open(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, hostfs.Private.File())
	if err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	closeErr := l.closeLocked()
	l.file = f
	l.encoder = json.NewEncoder(f)
	return closeErr
}

// Close closes the file, after which nothing is recorded; closing a closed Log does nothing.
func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.closeLocked()
}

// Record writes `command args`, which ended with err after d and must hold no secrets, as one UTC line.
func (l *Log) Record(command string, args []string, err error, d time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.encoder == nil {
		return
	}

	entry := Entry{
		Time:           time.Now().UTC(),
		Command:        command,
		Args:           args,
		DurationMillis: d.Milliseconds(),
	}
	entry.ExitCode, entry.Error = exitStatus(err)
	_ = l.encoder.Encode(entry)
}

func (l *Log) closeLocked() error {
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	l.encoder = nil
	return err
}

// exitStatus returns err's exit code, 0 for nil, or -1 with err's text when the command didn't exit.
func exitStatus(err error) (int, string) {
	if err == nil {
		return 0, ""
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() >= 0 {
		return exitErr.ExitCode(), ""
	}
	return -1, err.Error()
}
