package msb

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeRunner is the runner of a newTestClient; unset funcs fail like a missing msb binary.
type fakeRunner struct {
	run            func(r io.Reader, args ...string) (string, error)
	runInteractive func(args ...string) error
}

func (f *fakeRunner) Run(r io.Reader, args ...string) (string, error) {
	if f.run == nil {
		return "", fmt.Errorf("fake: unexpected msb %v", args)
	}
	return f.run(r, args...)
}

func (f *fakeRunner) RunInteractive(args ...string) error {
	if f.runInteractive == nil {
		return fmt.Errorf("fake: unexpected msb %v", args)
	}
	return f.runInteractive(args...)
}

// newTestClient returns a Client backed by f.
func newTestClient(f *fakeRunner) *Client {
	return &Client{runner: f}
}

// stubMsb writes a shell script named "msb" to a temp dir and prepends it to PATH.
func stubMsb(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "msb")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"+script+"\n"), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
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

// argAfter returns the value immediately following flag in args, or "".
func argAfter(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

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
