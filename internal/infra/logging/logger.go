package logging

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
	"golang.org/x/term"
)

const (
	prefix   = "agentic: "
	dimStart = "\x1b[2m"
	dimEnd   = "\x1b[0m"
)

// Logger writes messages to the user on its Streams' Err: steps, details and agentic-prefixed lines. Safe for
// concurrent use.
type Logger struct {
	mutex   sync.Mutex
	streams *iostreams.Streams
	color   bool

	// pending is set once an agentic-prefixed line is written and cleared by Blank.
	pending bool
}

// New returns a Logger writing to s.Err, dimming the agentic prefix when Err is a terminal.
func New(s *iostreams.Streams) *Logger {
	return &Logger{
		streams: s,
		color:   useColor(s.Err),
	}
}

// Step writes an "=>" item line.
func (l *Logger) Step(name string) {
	l.write("=> "+name+"\n", false)
}

// Stepf writes a formatted "=>" item line.
func (l *Logger) Stepf(format string, args ...any) {
	l.Step(fmt.Sprintf(format, args...))
}

// Detail writes an indented line under a step.
func (l *Logger) Detail(msg string) {
	l.write("   "+msg+"\n", false)
}

// Detailf writes a formatted indented line under a step.
func (l *Logger) Detailf(format string, args ...any) {
	l.Detail(fmt.Sprintf(format, args...))
}

// Info writes an agentic-prefixed message.
func (l *Logger) Info(msg string) {
	l.write(msg+"\n", true)
}

// Infof writes a formatted agentic-prefixed message.
func (l *Logger) Infof(format string, args ...any) {
	l.Info(fmt.Sprintf(format, args...))
}

// Error writes an agentic-prefixed error, for the failure that ends the command.
func (l *Logger) Error(msg string) {
	l.write("error: "+msg+"\n", true)
}

// Errorf writes a formatted agentic-prefixed error.
func (l *Logger) Errorf(format string, args ...any) {
	l.Error(fmt.Sprintf(format, args...))
}

// Warn writes an agentic-prefixed warning.
func (l *Logger) Warn(msg string) {
	l.write("warning: "+msg+"\n", true)
}

// Warnf writes a formatted agentic-prefixed warning.
func (l *Logger) Warnf(format string, args ...any) {
	l.Warn(fmt.Sprintf(format, args...))
}

// Blank writes a blank line; Separate is the conditional form.
func (l *Logger) Blank() {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.blank()
}

// Separate writes a blank line if agentic-prefixed lines came since the last one.
func (l *Logger) Separate() {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.pending {
		l.blank()
	}
}

// Prompt writes an agentic-prefixed question with no trailing newline.
func (l *Logger) Prompt(msg string) {
	l.write(msg, true)
}

// Promptf writes a formatted agentic-prefixed question with no trailing newline.
func (l *Logger) Promptf(format string, args ...any) {
	l.Prompt(fmt.Sprintf(format, args...))
}

// write writes text to Err under the lock, after the agentic prefix when prefixed.
func (l *Logger) write(text string, prefixed bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if prefixed {
		l.pending = true
		text = l.label() + text
	}
	fmt.Fprint(l.streams.Err, text)
}

// blank writes a blank line and clears pending; the caller holds the lock.
func (l *Logger) blank() {
	fmt.Fprintln(l.streams.Err)
	l.pending = false
}

// label returns the agentic prefix, dimmed when color is on.
func (l *Logger) label() string {
	if l.color {
		return dimStart + prefix + dimEnd
	}
	return prefix
}

// useColor reports whether w is a terminal and NO_COLOR is unset or empty.
func useColor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}
