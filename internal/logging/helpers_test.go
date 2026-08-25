package logging

import (
	"bytes"
	"testing"
)

// stubLog redirects Log to a buffer for the duration of the test and returns it.
func stubLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	orig := Log
	Log = New(&buf)
	t.Cleanup(func() { Log = orig })

	return &buf
}
