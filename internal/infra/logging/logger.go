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

// Logger writes messages to the user: steps, details and agentic-prefixed lines.
type Logger struct {
	w     io.Writer
	color bool

	// pending is set once an agentic-prefixed line is written and cleared by Blank.
	pending bool
}

// New returns a Logger that writes to w, dimming the agentic prefix when w is a terminal.
func New(w io.Writer) *Logger {
	return &Logger{w: w, color: useColor(w)}
}

// Step writes an "=>" item line.
func (l *Logger) Step(name string) {
	fmt.Fprintf(l.w, "=> %s\n", name)
}

// Stepf writes a formatted "=>" item line.
func (l *Logger) Stepf(format string, args ...any) {
	fmt.Fprintf(l.w, "=> "+format+"\n", args...)
}

// Detail writes an indented line under a step.
func (l *Logger) Detail(msg string) {
	fmt.Fprintf(l.w, "   %s\n", msg)
}

// Detailf writes a formatted indented line under a step.
func (l *Logger) Detailf(format string, args ...any) {
	fmt.Fprintf(l.w, "   "+format+"\n", args...)
}

// Infof writes an agentic-prefixed message.
func (l *Logger) Infof(format string, args ...any) {
	l.pending = true
	fmt.Fprintf(l.w, l.label()+format+"\n", args...)
}

// Errorf writes an agentic-prefixed error, for the failure that ends the command.
func (l *Logger) Errorf(format string, args ...any) {
	l.pending = true
	fmt.Fprintf(l.w, l.label()+"error: "+format+"\n", args...)
}

// Warnf writes an agentic-prefixed warning.
func (l *Logger) Warnf(format string, args ...any) {
	l.pending = true
	fmt.Fprintf(l.w, l.label()+"warning: "+format+"\n", args...)
}

// Blank writes a blank line; Separate is the conditional form.
func (l *Logger) Blank() {
	fmt.Fprintln(l.w)
	l.pending = false
}

// Separate writes a blank line if agentic-prefixed lines came since the last one.
func (l *Logger) Separate() {
	if l.pending {
		l.Blank()
	}
}

// Promptf writes an agentic-prefixed question with no trailing newline.
func (l *Logger) Promptf(format string, args ...any) {
	l.pending = true
	fmt.Fprintf(l.w, l.label()+format, args...)
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
