package logging

import "os"

// Log is the package-level Logger used by the Step/Detail free functions below.
var Log = New(os.Stdout)

// Err is the package-level Logger for agentic-prefixed run messages, kept on stderr so they stay off the tool's stdout.
var Err = New(os.Stderr)

func Step(name string) {
	Log.Step(name)
}

func Stepf(format string, args ...any) {
	Log.Stepf(format, args...)
}

func Detail(msg string) {
	Log.Detail(msg)
}

func Detailf(format string, args ...any) {
	Log.Detailf(format, args...)
}

func Infof(format string, args ...any) {
	Err.Infof(format, args...)
}

func Warnf(format string, args ...any) {
	Err.Warnf(format, args...)
}

func Separate() {
	Err.Separate()
}

func Promptf(format string, args ...any) {
	Err.Promptf(format, args...)
}
