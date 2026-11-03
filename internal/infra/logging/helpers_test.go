package logging

import (
	"bytes"
	"testing"
)

// stubOut redirects Out to a buffer for the duration of the test and returns it.
func stubOut(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	orig := Out
	Out = &buf
	t.Cleanup(func() { Out = orig })

	return &buf
}

// stubErr redirects Err to a buffer for the duration of the test and returns it.
func stubErr(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	orig := Err
	Err = New(&buf)
	t.Cleanup(func() { Err = orig })

	return &buf
}
