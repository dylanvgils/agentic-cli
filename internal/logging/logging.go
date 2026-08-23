package logging

import "os"

// Log is the package-level Logger used by the free functions below.
var Log = New(os.Stdout)

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
