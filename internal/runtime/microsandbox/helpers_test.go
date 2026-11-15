package microsandbox

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/require"
)

// testRunID is the run ID of test runs.
const testRunID sandbox.RunID = "0123456789ab"

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

// msbCall records what a stubbed RunInteractive got; conf is the --runtime-conf file's content, read while it exists.
type msbCall struct {
	env      []string
	args     []string
	confPath string
	conf     string
}

// stubRun returns a Client whose Run records its args and returns out and err.
func stubRun(out string, err error) (*Client, *[]string) {
	var args []string
	client := newTestClient(&fakeRunner{run: func(a ...string) (string, error) {
		args = a
		return out, err
	}})
	return client, &args
}

// stubRunInteractive returns a Client whose RunInteractive records its call and succeeds.
func stubRunInteractive() (*Client, *msbCall) {
	call := &msbCall{}
	client := newTestClient(&fakeRunner{runInteractive: func(env []string, args ...string) error {
		call.env, call.args = env, args
		for _, a := range args {
			if path, ok := strings.CutPrefix(a, "--runtime-conf="); ok {
				data, err := os.ReadFile(path)
				call.confPath, call.conf = path, string(data)
				return err
			}
		}
		return nil
	}})
	return client, call
}
