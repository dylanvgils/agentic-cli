// Package iostreams owns the process streams: In for input, Out for data, Err for messages and progress.
package iostreams

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"

	"golang.org/x/term"
)

// std is the Streams the package funcs use; empty input and discarded output until a main calls SetDefault.
var std atomic.Pointer[Streams]

// Streams is one set of process streams; treat it as immutable and replace the default with SetDefault.
type Streams struct {
	In  io.Reader // where input comes from, such as prompt answers
	Out io.Writer // where data goes: what a script would pipe, such as tables and dry-run output
	Err io.Writer // where messages and subprocess progress go, kept out of piped data
}

func init() {
	std.Store(New(strings.NewReader(""), io.Discard, io.Discard))
}

// New returns Streams over in, out and err.
func New(in io.Reader, out, err io.Writer) *Streams {
	return &Streams{
		In:  in,
		Out: out,
		Err: err,
	}
}

// System returns the process's own stdin, stdout and stderr, for the mains to make the default.
func System() *Streams {
	return New(os.Stdin, os.Stdout, os.Stderr) //nolint:forbidigo // the one place that names the process streams
}

// Print writes a to Out like fmt.Print, for data such as a Dockerfile.
func (s *Streams) Print(a ...any) {
	fmt.Fprint(s.Out, a...)
}

// Printf writes formatted data to Out.
func (s *Streams) Printf(format string, args ...any) {
	fmt.Fprintf(s.Out, format, args...)
}

// Println writes a and a newline to Out like fmt.Println; with no args it writes a blank line.
func (s *Streams) Println(a ...any) {
	fmt.Fprintln(s.Out, a...)
}

// IsTerminal reports whether In is an interactive terminal.
func (s *Streams) IsTerminal() bool {
	f, ok := s.In.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// Default returns the Streams the package funcs use.
func Default() *Streams {
	return std.Load()
}

// SetDefault makes s the Streams the package funcs use; mains call it first with System.
func SetDefault(s *Streams) {
	std.Store(s)
}

// Print calls Default().Print.
func Print(a ...any) {
	Default().Print(a...)
}

// Printf calls Default().Printf.
func Printf(format string, args ...any) {
	Default().Printf(format, args...)
}

// Println calls Default().Println.
func Println(a ...any) {
	Default().Println(a...)
}

// IsTerminal calls Default().IsTerminal.
func IsTerminal() bool {
	return Default().IsTerminal()
}
