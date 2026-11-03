package iostreams

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// newTestStreams returns Streams with empty input, discarded Err and Out on the returned buffer.
func newTestStreams() (*Streams, *bytes.Buffer) {
	var out bytes.Buffer
	return New(strings.NewReader(""), &out, io.Discard), &out
}

// stubDefault makes test Streams the default for the duration of the test and returns its Out buffer.
func stubDefault(t *testing.T) *bytes.Buffer {
	t.Helper()

	orig := Default()
	t.Cleanup(func() { std.Store(orig) })

	s, out := newTestStreams()
	std.Store(s)
	return out
}
