package docker

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/require"
)

// dockerCall records a single run invocation.
type dockerCall struct {
	args []string
}

// dockerStdinCall records a single runStdin invocation.
type dockerStdinCall struct {
	args  []string
	input []byte
}

// tarFile is one file read back from a tar archive.
type tarFile struct {
	header  *tar.Header
	content []byte
}

// fakeRunner is the runner of a newTestClient; unset funcs fail like a missing docker binary.
type fakeRunner struct {
	run            func(args ...string) (string, error)
	runStdin       func(r io.Reader, args ...string) (string, error)
	runInteractive func(args ...string) error
}

func (f *fakeRunner) Run(r io.Reader, args ...string) (string, error) {
	if r != nil {
		if f.runStdin == nil {
			return "", fmt.Errorf("fake: unexpected docker %v", args)
		}
		return f.runStdin(r, args...)
	}
	if f.run == nil {
		return "", fmt.Errorf("fake: unexpected docker %v", args)
	}
	return f.run(args...)
}

func (f *fakeRunner) RunInteractive(args ...string) error {
	if f.runInteractive == nil {
		return fmt.Errorf("fake: unexpected docker %v", args)
	}
	return f.runInteractive(args...)
}

// stubDocker writes a shell script named "docker" to a temp dir and prepends it to PATH.
func stubDocker(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "docker")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"+script+"\n"), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// newTestClient returns a Client backed by a fakeRunner, for the stub helpers below.
func newTestClient() *Client {
	return &Client{runner: &fakeRunner{}}
}

// fakeOf returns the fakeRunner behind a newTestClient.
func fakeOf(client *Client) *fakeRunner {
	return client.runner.(*fakeRunner)
}

// stubDockerRun replaces client.run with fn for the duration of the test.
func stubDockerRun(t *testing.T, client *Client, fn func(...string) (string, error)) {
	t.Helper()
	f := fakeOf(client)
	orig := f.run
	f.run = fn
	t.Cleanup(func() { f.run = orig })
}

// stubDockerRunFixed stubs client.run to always return a fixed output and error.
func stubDockerRunFixed(t *testing.T, client *Client, output string, err error) {
	t.Helper()
	stubDockerRun(t, client, func(_ ...string) (string, error) { return output, err })
}

// stubDockerRunBySubcmd stubs client.run, routing by first arg.
func stubDockerRunBySubcmd(t *testing.T, client *Client, responses map[string]string) {
	t.Helper()
	stubDockerRun(t, client, func(args ...string) (string, error) {
		if out, ok := responses[args[0]]; ok {
			return out, nil
		}
		return "", nil
	})
}

// stubDockerRunCapture replaces client.run with a stub that records calls and fails any
// "verb sub" pair (e.g. "volume inspect") listed in failSubcmds.
func stubDockerRunCapture(t *testing.T, client *Client, failSubcmds ...string) func() []dockerCall {
	t.Helper()
	var calls []dockerCall
	failing := make(map[string]bool, len(failSubcmds))
	for _, s := range failSubcmds {
		failing[s] = true
	}

	stubDockerRun(t, client, func(args ...string) (string, error) {
		calls = append(calls, dockerCall{args: args})
		key := args[0]
		if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
			key += " " + args[1]
		}
		if failing[key] {
			return "", fmt.Errorf("stub: %s failed", key)
		}
		return "", nil
	})

	return func() []dockerCall { return calls }
}

// stubRunInteractive replaces client.runInteractive with a mock that records the args of the most recent call.
func stubRunInteractive(t *testing.T, client *Client) func() []string {
	t.Helper()
	var capturedArgs []string

	stubRunInteractiveFunc(t, client, func(args ...string) error {
		capturedArgs = args
		return nil
	})

	return func() []string { return capturedArgs }
}

// stubRunInteractiveFunc replaces client.runInteractive with fn for the duration of the test.
func stubRunInteractiveFunc(t *testing.T, client *Client, fn func(...string) error) {
	t.Helper()
	f := fakeOf(client)
	orig := f.runInteractive
	f.runInteractive = fn
	t.Cleanup(func() { f.runInteractive = orig })
}

// stubIsTerminal replaces isTerminal with a stub that returns val for the duration of the test.
func stubIsTerminal(t *testing.T, val bool) {
	t.Helper()
	orig := isTerminal
	isTerminal = func() bool { return val }
	t.Cleanup(func() { isTerminal = orig })
}

// stubHostTimezone replaces hostTimezone with a stub that returns val for the duration of the test.
func stubHostTimezone(t *testing.T, val string) {
	t.Helper()
	orig := hostTimezone
	hostTimezone = func() string { return val }
	t.Cleanup(func() { hostTimezone = orig })
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

// stubRunInteractiveCapture captures the last client.runInteractive call's args and, if present, its
// --file Dockerfile content (read inside the stub before the temp file is removed).
func stubRunInteractiveCapture(t *testing.T, client *Client) func() (args []string, dockerfile string) {
	t.Helper()
	var capturedArgs []string
	var content string

	stubRunInteractiveFunc(t, client, func(a ...string) error {
		capturedArgs = a
		for _, x := range a {
			if p, ok := strings.CutPrefix(x, "--file="); ok {
				b, err := os.ReadFile(p)
				require.NoError(t, err)
				content = string(b)
			}
		}
		return nil
	})

	return func() ([]string, string) { return capturedArgs, content }
}

// stubLatestVersion replaces tool's Build.LatestVersion fetcher for the duration of the test.
func stubLatestVersion(t *testing.T, tool string, fn func() (string, error)) {
	t.Helper()
	orig := tools.Configs[tool]
	cfg := orig
	cfg.Build.LatestVersion = fn
	tools.Configs[tool] = cfg
	t.Cleanup(func() { tools.Configs[tool] = orig })
}

// hasArg reports whether args contains exactly value.
func hasArg(args []string, value string) bool {
	return slices.Contains(args, value)
}

// hasArgWithPrefix reports whether any arg in args starts with prefix.
func hasArgWithPrefix(args []string, prefix string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			return true
		}
	}
	return false
}

// stubDindReadyTimeout shortens the sidecar readiness wait so failure paths finish quickly.
func stubDindReadyTimeout(t *testing.T) {
	t.Helper()
	origTimeout, origInterval := dindReadyTimeout, dindPollInterval
	dindReadyTimeout, dindPollInterval = 0, 0
	t.Cleanup(func() { dindReadyTimeout, dindPollInterval = origTimeout, origInterval })
}

// stubDindReadyTimeoutLong makes the readiness wait long enough that only a fail-fast path ends it.
func stubDindReadyTimeoutLong(t *testing.T) {
	t.Helper()
	orig := dindReadyTimeout
	dindReadyTimeout = time.Hour
	t.Cleanup(func() { dindReadyTimeout = orig })
}

// stubDindProbe stubs client.run so the readiness probe always fails and `inspect` reports running as the given state.
func stubDindProbe(t *testing.T, client *Client, running string) func() []dockerCall {
	t.Helper()
	var calls []dockerCall
	stubDockerRun(t, client, func(args ...string) (string, error) {
		calls = append(calls, dockerCall{args: args})
		switch args[0] {
		case "exec":
			return "", fmt.Errorf("stub: dockerd not ready")
		case "inspect":
			return running + "\n", nil
		case "logs":
			return "rootlesskit: failed to setup UID/GID map", nil
		}
		return "", nil
	})
	return func() []dockerCall { return calls }
}

// findCall returns the args of the first recorded call whose leading args equal prefix, or nil.
func findCall(calls []dockerCall, prefix ...string) []string {
	for _, c := range calls {
		if len(c.args) >= len(prefix) && slices.Equal(c.args[:len(prefix)], prefix) {
			return c.args
		}
	}
	return nil
}

// stubRunInteractiveError replaces client.runInteractive with a stub that always returns err.
func stubRunInteractiveError(t *testing.T, client *Client, err error) {
	t.Helper()
	stubRunInteractiveFunc(t, client, func(...string) error { return err })
}

// stubHostUserGroup replaces hostUserGroup with a stub that returns val for the duration of the test.
func stubHostUserGroup(t *testing.T, val string) {
	t.Helper()
	orig := hostUserGroup
	hostUserGroup = func() string { return val }
	t.Cleanup(func() { hostUserGroup = orig })
}

// stubDindRunDirGrace replaces dindRunDirGrace with d for the duration of the test.
func stubDindRunDirGrace(t *testing.T, d time.Duration) {
	t.Helper()
	orig := dindRunDirGrace
	dindRunDirGrace = d
	t.Cleanup(func() { dindRunDirGrace = orig })
}

// makeDindRunDir creates toolHome/dind/<name> holding a key file, backdated by age, and returns its path.
func makeDindRunDir(t *testing.T, toolHome, name string, age time.Duration) string {
	t.Helper()
	dir := filepath.Join(toolHome, dindDirName, name)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "key.pem"), []byte("key"), 0o600))
	past := time.Now().Add(-age)
	require.NoError(t, os.Chtimes(dir, past, past))
	return dir
}

// stubSidecarOrphanGrace replaces sidecarOrphanGrace with d for the duration of the test.
func stubSidecarOrphanGrace(t *testing.T, d time.Duration) {
	t.Helper()
	orig := sidecarOrphanGrace
	sidecarOrphanGrace = d
	t.Cleanup(func() { sidecarOrphanGrace = orig })
}

// stubOwnedResources stubs `ps` and `network ls` output, recording every call.
func stubOwnedResources(t *testing.T, client *Client, containers, networks string) func() []dockerCall {
	t.Helper()
	var calls []dockerCall
	stubDockerRun(t, client, func(args ...string) (string, error) {
		calls = append(calls, dockerCall{args: args})
		switch {
		case args[0] == "ps":
			return containers, nil
		case args[0] == "network" && args[1] == "ls":
			return networks, nil
		}
		return "", nil
	})
	return func() []dockerCall { return calls }
}

// ownedRow formats a row as ownedFormat renders it.
func ownedRow(name, owner string, age time.Duration) string {
	return name + "\t" + owner + "\t" + formatLabelTime(time.Now().Add(-age)) + "\n"
}

// stubDockerRunStdin replaces client.runStdin with fn for the duration of the test.
func stubDockerRunStdin(t *testing.T, client *Client, fn func(io.Reader, ...string) (string, error)) {
	t.Helper()
	f := fakeOf(client)
	orig := f.runStdin
	f.runStdin = fn
	t.Cleanup(func() { f.runStdin = orig })
}

// stubDockerRunStdinCapture stubs client.runStdin to return err, recording each call's args and input.
func stubDockerRunStdinCapture(t *testing.T, client *Client, err error) func() []dockerStdinCall {
	t.Helper()
	var calls []dockerStdinCall
	stubDockerRunStdin(t, client, func(r io.Reader, args ...string) (string, error) {
		input, readErr := io.ReadAll(r)
		require.NoError(t, readErr)
		calls = append(calls, dockerStdinCall{args: args, input: input})
		return "", err
	})
	return func() []dockerStdinCall { return calls }
}

// readTar returns each regular file in archive by name, with its header.
func readTar(t *testing.T, archive []byte) map[string]tarFile {
	t.Helper()
	files := make(map[string]tarFile)
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files
		}
		require.NoError(t, err)
		content, err := io.ReadAll(tr)
		require.NoError(t, err)
		files[header.Name] = tarFile{header: header, content: content}
	}
}

// stubProxyDocker records client.run calls, answering network inspect with a test subnet and container inspect with running.
func stubProxyDocker(t *testing.T, client *Client, running string) func() []dockerCall {
	t.Helper()
	var calls []dockerCall
	stubDockerRun(t, client, func(args ...string) (string, error) {
		calls = append(calls, dockerCall{args: args})
		switch {
		case len(args) > 1 && args[0] == "network" && args[1] == "inspect":
			return "172.30.0.0/16 \n", nil
		case args[0] == "inspect":
			return running, nil
		}
		return "", nil
	})
	return func() []dockerCall { return calls }
}

// stubProxySettleTime replaces proxySettleTime with d for the duration of the test.
func stubProxySettleTime(t *testing.T, d time.Duration) {
	t.Helper()
	orig := proxySettleTime
	proxySettleTime = d
	t.Cleanup(func() { proxySettleTime = orig })
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
