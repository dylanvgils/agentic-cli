// Package logging writes messages to the user: steps, details and agentic-prefixed lines, on Err only. Data goes
// through iostreams instead, so message filtering can never touch it. The package funcs use Default.
package logging

import (
	"sync/atomic"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
)

// std is the Logger the package funcs use, over the iostreams default it was built for.
var std atomic.Pointer[Logger]

// Default returns the Logger the package funcs use, building a new one when iostreams.Default changed.
func Default() *Logger {
	s := iostreams.Default()
	for {
		l := std.Load()
		if l != nil && l.streams == s {
			return l
		}
		if next := New(s); std.CompareAndSwap(l, next) {
			return next
		}
	}
}

// Step calls Default().Step.
func Step(name string) {
	Default().Step(name)
}

// Stepf calls Default().Stepf.
func Stepf(format string, args ...any) {
	Default().Stepf(format, args...)
}

// Detail calls Default().Detail.
func Detail(msg string) {
	Default().Detail(msg)
}

// Detailf calls Default().Detailf.
func Detailf(format string, args ...any) {
	Default().Detailf(format, args...)
}

// Info calls Default().Info.
func Info(msg string) {
	Default().Info(msg)
}

// Infof calls Default().Infof.
func Infof(format string, args ...any) {
	Default().Infof(format, args...)
}

// Error calls Default().Error.
func Error(msg string) {
	Default().Error(msg)
}

// Errorf calls Default().Errorf.
func Errorf(format string, args ...any) {
	Default().Errorf(format, args...)
}

// Warn calls Default().Warn.
func Warn(msg string) {
	Default().Warn(msg)
}

// Warnf calls Default().Warnf.
func Warnf(format string, args ...any) {
	Default().Warnf(format, args...)
}

// Blank calls Default().Blank.
func Blank() {
	Default().Blank()
}

// Separate calls Default().Separate.
func Separate() {
	Default().Separate()
}

// Prompt calls Default().Prompt.
func Prompt(msg string) {
	Default().Prompt(msg)
}

// Promptf calls Default().Promptf.
func Promptf(format string, args ...any) {
	Default().Promptf(format, args...)
}
