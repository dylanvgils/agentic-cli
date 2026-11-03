// Package logging writes CLI output: data to stdout through Out, messages to stderr through Err.
package logging

import (
	"fmt"
	"io"
	"os"
)

var (
	// Out is where data goes: what a script would pipe, such as tables and dry-run output.
	Out io.Writer = os.Stdout

	// Err is the Logger for messages, kept on stderr so they stay out of piped data.
	Err = New(os.Stderr)
)

// Print writes s to Out unchanged, for data such as a Dockerfile.
func Print(s string) {
	fmt.Fprint(Out, s)
}

// Println writes s and a newline to Out.
func Println(s string) {
	fmt.Fprintln(Out, s)
}

// Step calls Err.Step.
func Step(name string) {
	Err.Step(name)
}

// Stepf calls Err.Stepf.
func Stepf(format string, args ...any) {
	Err.Stepf(format, args...)
}

// Detail calls Err.Detail.
func Detail(msg string) {
	Err.Detail(msg)
}

// Detailf calls Err.Detailf.
func Detailf(format string, args ...any) {
	Err.Detailf(format, args...)
}

// Infof calls Err.Infof.
func Infof(format string, args ...any) {
	Err.Infof(format, args...)
}

// Errorf calls Err.Errorf.
func Errorf(format string, args ...any) {
	Err.Errorf(format, args...)
}

// Warnf calls Err.Warnf.
func Warnf(format string, args ...any) {
	Err.Warnf(format, args...)
}

// Separate calls Err.Separate.
func Separate() {
	Err.Separate()
}

// Promptf calls Err.Promptf.
func Promptf(format string, args ...any) {
	Err.Promptf(format, args...)
}
