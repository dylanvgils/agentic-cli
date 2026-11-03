package sandbox

import (
	"bytes"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
)

// clearTerminalEnv unsets the forwarded terminal vars for the duration of the test.
func clearTerminalEnv(t *testing.T) {
	t.Helper()
	for _, key := range terminalEnvNames {
		t.Setenv(key, "")
	}
}

// stubLogs routes agentic-prefixed messages into the returned buffer for the duration of the test.
func stubLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := logging.Err
	logging.Err = logging.New(&buf)
	t.Cleanup(func() { logging.Err = orig })
	return &buf
}

// captureOut swaps logging.Out for the duration of fn and returns what was written.
func captureOut(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer
	orig := logging.Out
	logging.Out = &buf
	t.Cleanup(func() { logging.Out = orig })

	fn()

	return buf.String()
}
