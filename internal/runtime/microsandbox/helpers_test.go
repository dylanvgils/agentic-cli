package microsandbox

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
	run            func(args ...string) (string, error)
	runWithStdin   func(r io.Reader, args ...string) (string, error)
	runInteractive func(env []string, args ...string) error
}

func (f *fakeRunner) Run(args ...string) (string, error) {
	if f.run == nil {
		return "", fmt.Errorf("fake: unexpected msb %v", args)
	}
	return f.run(args...)
}

func (f *fakeRunner) RunWithStdin(r io.Reader, args ...string) (string, error) {
	if f.runWithStdin == nil {
		return "", fmt.Errorf("fake: unexpected msb %v", args)
	}
	return f.runWithStdin(r, args...)
}

func (f *fakeRunner) RunInteractive(env []string, args ...string) error {
	if f.runInteractive == nil {
		return fmt.Errorf("fake: unexpected msb %v", args)
	}
	return f.runInteractive(env, args...)
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
