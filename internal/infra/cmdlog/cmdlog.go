// Package cmdlog records the commands agentic runs, such as docker and msb calls, as JSON lines in a log file.
package cmdlog

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/infra/hostfs"
)

// maxReasonFrames is how deep Reason looks up the stack for the runtime package's frames.
const maxReasonFrames = 128

// closureSuffix matches the parts Go appends to a closure's name, such as func1, gowrap2 or a nested closure's 1.
var closureSuffix = regexp.MustCompile(`^(func|gowrap)?\d+$`)

// Entry is one command, written as one JSON line like the proxy's access log.
type Entry struct {
	Time    time.Time `json:"time"`
	Command string    `json:"command"`
	// Args must hold no secrets; the caller masks them.
	Args []string `json:"args"`
	// Reason is why the command ran: the runtime package's methods that led to it, outermost first.
	Reason string `json:"reason,omitempty"`
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

// Record writes `command args`, run for reason, which ended with err after d and must hold no secrets, as one UTC line.
func (l *Log) Record(command string, args []string, reason string, err error, d time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.encoder == nil {
		return
	}

	entry := Entry{
		Time:           time.Now().UTC(),
		Command:        command,
		Args:           args,
		Reason:         reason,
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

// Reason returns the methods of pkgPath on the caller's stack, outermost first and joined by " > ", skipping plumbing
// names such as "Client.run" and receivers ending in "." such as "execRunner.", and an inner method that only repeats
// its outer one's name in another case; empty when pkgPath isn't on the stack.
func Reason(pkgPath string, plumbing ...string) string {
	pcs := make([]uintptr, maxReasonFrames)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])

	var methods []string
	for {
		frame, more := frames.Next()
		if name, ok := frameName(frame.Function, pkgPath); ok && !isPlumbing(name, plumbing) {
			methods = append(methods, name[strings.LastIndex(name, ".")+1:])
		}
		if !more {
			break
		}
	}

	var reason []string
	for i := len(methods) - 1; i >= 0; i-- {
		if len(reason) == 0 || !strings.EqualFold(methods[i], reason[len(reason)-1]) {
			reason = append(reason, methods[i])
		}
	}
	return strings.Join(reason, " > ")
}

// frameName returns function as Type.method or func, without its package or closure suffixes; false outside pkgPath.
func frameName(function, pkgPath string) (string, bool) {
	rest, ok := strings.CutPrefix(function, pkgPath+".")
	if !ok {
		return "", false
	}

	parts := strings.Split(strings.NewReplacer("(*", "", ")", "").Replace(rest), ".")
	for len(parts) > 1 && closureSuffix.MatchString(parts[len(parts)-1]) {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, "."), true
}

// isPlumbing reports whether name is in plumbing, or its receiver is, as an entry ending in ".".
func isPlumbing(name string, plumbing []string) bool {
	for _, p := range plumbing {
		if name == p || (strings.HasSuffix(p, ".") && strings.HasPrefix(name, p)) {
			return true
		}
	}
	return false
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
