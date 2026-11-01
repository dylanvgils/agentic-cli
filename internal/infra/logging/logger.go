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

// Print writes s unchanged, for data such as a Dockerfile or shell lines rather than a message.
func (l *Logger) Print(s string) {
	fmt.Fprint(l.w, s)
}

// Println writes s and a newline, unchanged otherwise.
func (l *Logger) Println(s string) {
	fmt.Fprintln(l.w, s)
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

// Blank writes a blank line, setting what follows apart from output the logger didn't write (e.g. a tool's); Separate is the conditional form.
func (l *Logger) Blank() {
	fmt.Fprintln(l.w)
	l.pending = false
}

// Separate writes a blank line if agentic-prefixed lines were written since the last call, setting them apart from what follows.
func (l *Logger) Separate() {
	if l.pending {
		l.Blank()
	}
}

// Promptf writes an agentic-prefixed question with no trailing newline, so the answer is typed on the same line.
func (l *Logger) Promptf(format string, args ...any) {
	l.pending = true
	fmt.Fprintf(l.w, l.prefix()+format, args...)
}

// Writer returns the underlying destination, for helpers that need an io.Writer (e.g. a tabwriter table).
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
