package microsandbox

import (
	"fmt"
	"io"
	"os"
	"os/exec"
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

// stubRunTempDir points the OS temp dir at a test dir and creates testRunID's run temp dir in it.
func stubRunTempDir(t *testing.T) {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	require.NoError(t, sandbox.CreateRunTempDir(testRunID))
}

// fakeImageSource is an ImageSource; a nil imageID finds every image as abc123, a nil saveImage writes "tar:<image>".
type fakeImageSource struct {
	imageID   func(image string) (string, bool, error)
	saveImage func(image string, w io.Writer) error
}

func (f *fakeImageSource) ImageID(image string) (string, bool, error) {
	if f.imageID == nil {
		return "abc123", true, nil
	}
	return f.imageID(image)
}

func (f *fakeImageSource) SaveImage(image string, w io.Writer) error {
	if f.saveImage == nil {
		_, err := io.WriteString(w, "tar:"+image)
		return err
	}
	return f.saveImage(image, w)
}

// fakeImageStore stubs msb's image commands, recording each as "inspect <ref>", "load <ref>", "images" or "rmi <ref>";
// msb holds no ref unless held, and an error field fails its command.
type fakeImageStore struct {
	held       bool
	inspectErr error
	loadErr    error
	refs       []string
	refsErr    error
	rmiErrs    map[string]error
	calls      []string
}

// client returns a Client backed by s.
func (s *fakeImageStore) client() *Client {
	return newTestClient(&fakeRunner{
		run: func(args ...string) (string, error) {
			switch args[0] {
			case "image":
				s.calls = append(s.calls, "inspect "+args[2])
				return "", s.inspectResult()
			case "images":
				s.calls = append(s.calls, "images")
				return strings.Join(s.refs, "\n"), s.refsErr
			case "rmi":
				s.calls = append(s.calls, "rmi "+args[1])
				return "", s.rmiErrs[args[1]]
			}
			return "", fmt.Errorf("fake: unexpected msb %v", args)
		},
		runWithStdin: func(r io.Reader, args ...string) (string, error) {
			s.calls = append(s.calls, "load "+strings.TrimPrefix(args[1], "--tag="))
			_, _ = io.ReadAll(r)
			return "", s.loadErr
		},
	})
}

// inspectResult is what `msb image inspect` returns: nil when held, a non-zero exit when missing.
func (s *fakeImageStore) inspectResult() error {
	if s.inspectErr != nil {
		return s.inspectErr
	}
	if s.held {
		return nil
	}
	return exec.Command("false").Run()
}
