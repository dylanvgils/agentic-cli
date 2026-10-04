package logging

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

const (
	prefix   = "agentic: "
	dimStart = "\x1b[2m"
	dimEnd   = "\x1b[0m"
)

// Logger writes formatted progress/status messages to a destination.
type Logger struct {
	w     io.Writer
	color bool

	// pending is set once an agentic-prefixed line is written and cleared by Separate.
	pending bool
}

// New returns a Logger that writes to w, dimming the agentic prefix when w is a terminal.
func New(w io.Writer) *Logger {
	return &Logger{w: w, color: useColor(w)}
}

func (l *Logger) Step(name string) {
	fmt.Fprintf(l.w, "=> %s\n", name)
}

func (l *Logger) Stepf(format string, args ...any) {
	fmt.Fprintf(l.w, "=> "+format+"\n", args...)
}

func (l *Logger) Detail(msg string) {
	fmt.Fprintf(l.w, "   %s\n", msg)
}

func (l *Logger) Detailf(format string, args ...any) {
	fmt.Fprintf(l.w, "   "+format+"\n", args...)
}

// Infof writes an agentic-prefixed message, marking it as agentic's own output amid a tool's.
func (l *Logger) Infof(format string, args ...any) {
	l.pending = true
	fmt.Fprintf(l.w, l.prefix()+format+"\n", args...)
}

// Warnf writes an agentic-prefixed warning.
func (l *Logger) Warnf(format string, args ...any) {
	l.pending = true
	fmt.Fprintf(l.w, l.prefix()+"warning: "+format+"\n", args...)
}

// Separate writes a blank line if agentic-prefixed lines were written since the last call, setting them apart from what follows.
func (l *Logger) Separate() {
	if !l.pending {
		return
	}

	fmt.Fprintln(l.w)
	l.pending = false
}

// Promptf writes an agentic-prefixed question with no trailing newline, so the answer is typed on the same line.
func (l *Logger) Promptf(format string, args ...any) {
	l.pending = true
	fmt.Fprintf(l.w, l.prefix()+format, args...)
}

// Writer returns the underlying destination, for a message shape Logger's methods don't cover (e.g. a prompt with no trailing newline).
func (l *Logger) Writer() io.Writer {
	return l.w
}

func (l *Logger) prefix() string {
	if l.color {
		return dimStart + prefix + dimEnd
	}
	return prefix
}

// useColor reports whether w is a terminal and NO_COLOR is unset.
func useColor(w io.Writer) bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}

	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}
