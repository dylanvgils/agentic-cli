package logging

import (
	"fmt"
	"io"
)

// Logger writes formatted progress/status messages to a destination.
type Logger struct {
	w io.Writer
}

// New returns a Logger that writes to w.
func New(w io.Writer) *Logger {
	return &Logger{w: w}
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

// Writer returns the underlying destination, for callers writing a message
// shape Logger's methods don't cover (e.g. a prompt with no trailing newline).
func (l *Logger) Writer() io.Writer {
	return l.w
}
