package logging

import (
	"bytes"
	"io"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
)

// newTestLogger returns a Logger whose Err is buf.
func newTestLogger(buf *bytes.Buffer) *Logger {
	return New(iostreams.New(strings.NewReader(""), io.Discard, buf))
}
