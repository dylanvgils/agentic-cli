// Package iostreamstest swaps the default iostreams in tests, restoring the original when the test ends.
package iostreamstest

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
)

// StubIn makes the default In read input for the duration of the test.
func StubIn(t testing.TB, input string) {
	t.Helper()

	swap(t, func(s *iostreams.Streams) *iostreams.Streams {
		return iostreams.New(strings.NewReader(input), s.Out, s.Err)
	})
}

// StubOut points the default Out at a buffer for the duration of the test and returns it.
func StubOut(t testing.TB) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	swap(t, func(s *iostreams.Streams) *iostreams.Streams {
		return iostreams.New(s.In, &buf, s.Err)
	})
	return &buf
}

// StubErr points the default Err at a buffer for the duration of the test and returns it.
func StubErr(t testing.TB) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	swap(t, func(s *iostreams.Streams) *iostreams.Streams {
		return iostreams.New(s.In, s.Out, &buf)
	})
	return &buf
}

// StubCombined points the default Out and Err at one buffer, for checking order across both, and returns it.
func StubCombined(t testing.TB) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	swap(t, func(s *iostreams.Streams) *iostreams.Streams {
		return iostreams.New(s.In, &buf, &buf)
	})
	return &buf
}

// CaptureOut returns what fn writes to the default Out.
func CaptureOut(t testing.TB, fn func()) string {
	t.Helper()

	buf := StubOut(t)
	fn()
	return buf.String()
}

// CaptureErr returns what fn writes to the default Err.
func CaptureErr(t testing.TB, fn func()) string {
	t.Helper()

	buf := StubErr(t)
	fn()
	return buf.String()
}

// swap makes next(current default) the default until the test ends.
func swap(t testing.TB, next func(*iostreams.Streams) *iostreams.Streams) {
	orig := iostreams.Default()
	iostreams.SetDefault(next(orig))
	t.Cleanup(func() { iostreams.SetDefault(orig) })
}
