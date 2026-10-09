package upgradecheck

import (
	"bytes"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/logging"
)

// stubBuildVersion sets buildinfo.Version for the duration of the test.
func stubBuildVersion(t *testing.T, v string) {
	t.Helper()
	orig := buildinfo.Version
	buildinfo.Version = v
	t.Cleanup(func() { buildinfo.Version = orig })
}

// stubExit records the exit code Exit was called with instead of exiting; -1 means it was never called.
func stubExit(t *testing.T) *int {
	t.Helper()
	code := -1
	orig := Exit
	Exit = func(c int) { code = c }
	t.Cleanup(func() { Exit = orig })
	return &code
}

// stubLatestVersion makes LatestVersion return v and err, reporting whether it was called.
func stubLatestVersion(t *testing.T, v string, err error) *bool {
	t.Helper()
	var called bool
	orig := LatestVersion
	LatestVersion = func() (string, error) {
		called = true
		return v, err
	}
	t.Cleanup(func() { LatestVersion = orig })
	return &called
}

// stubNotify redirects Notify to a buffer for the duration of the test.
func stubNotify(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := Notify
	Notify = logging.New(&buf)
	t.Cleanup(func() { Notify = orig })
	return &buf
}

// stubUpdate records the version Update was called with, returning err.
func stubUpdate(t *testing.T, err error) *string {
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
