package sandbox

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

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

// captureStdout returns what fn writes to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })

	fn()

	require.NoError(t, w.Close())
	os.Stdout = orig
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}
