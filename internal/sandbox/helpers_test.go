package sandbox

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dylanvgils/agentic-cli/internal/logging"
)

// stubHostTimezone replaces hostTimezone with a stub that returns val for the duration of the test.
func stubHostTimezone(t *testing.T, val string) {
	t.Helper()
	orig := hostTimezone
	hostTimezone = func() string { return val }
	t.Cleanup(func() { hostTimezone = orig })
}

// clearTerminalEnv unsets the forwarded terminal vars for the duration of the test.
func clearTerminalEnv(t *testing.T) {
	t.Helper()
	for _, key := range terminalEnvNames {
		t.Setenv(key, "")
	}
}

// stubHostUsesPowerShell replaces hostUsesPowerShell with a stub that returns val for the duration of the test.
func stubHostUsesPowerShell(t *testing.T, val bool) {
	t.Helper()
	orig := hostUsesPowerShell
	hostUsesPowerShell = func() bool { return val }
	t.Cleanup(func() { hostUsesPowerShell = orig })
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
