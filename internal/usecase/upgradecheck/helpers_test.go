package upgradecheck

import (
	"bytes"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/logging"
)

func stubLatestVersion(t *testing.T, v string, err error) {
	t.Helper()
	orig := LatestVersion
	LatestVersion = func() (string, error) { return v, err }
	t.Cleanup(func() { LatestVersion = orig })
}

func stubUpdate(t *testing.T, err error) {
	t.Helper()
	orig := Update
	Update = func(_ string) error { return err }
	t.Cleanup(func() { Update = orig })
}

// stubUpdateCapture records the version Update was called with, returning err.
func stubUpdateCapture(t *testing.T, err error) *string {
	t.Helper()
	var calledWith string
	orig := Update
	Update = func(v string) error {
		calledWith = v
		return err
	}
	t.Cleanup(func() { Update = orig })
	return &calledWith
}

func stubIsTerminal(t *testing.T, terminal bool) {
	t.Helper()
	orig := IsTerminal
	IsTerminal = func() bool { return terminal }
	t.Cleanup(func() { IsTerminal = orig })
}

// stubStderrCapture redirects Log to a buffer for the duration of the test.
func stubStderrCapture(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := Log
	Log = logging.New(&buf)
	t.Cleanup(func() { Log = orig })
	return &buf
}

// stubExitCapture records the exit code Exit was called with instead of exiting.
func stubExitCapture(t *testing.T) *int {
	t.Helper()
	var code int
	orig := Exit
	Exit = func(c int) { code = c }
	t.Cleanup(func() { Exit = orig })
	return &code
}
