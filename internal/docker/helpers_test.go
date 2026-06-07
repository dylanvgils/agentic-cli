package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// dockerCall records a single dockerRun invocation.
type dockerCall struct {
	args []string
}

// stubDocker writes a shell script named "docker" to a temp dir and prepends
// it to PATH. t.Setenv handles cleanup automatically.
func stubDocker(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "docker")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"+script+"\n"), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// stubDockerRun replaces dockerRun with fn for the duration of the test.
func stubDockerRun(t *testing.T, fn func(...string) (string, error)) {
	t.Helper()
	orig := dockerRun
	dockerRun = fn
	t.Cleanup(func() { dockerRun = orig })
}

// stubDockerRunFixed stubs dockerRun to always return a fixed output and error.
func stubDockerRunFixed(t *testing.T, output string, err error) {
	t.Helper()
	stubDockerRun(t, func(_ ...string) (string, error) { return output, err })
}

// stubDockerRunBySubcmd stubs dockerRun, routing by first arg.
func stubDockerRunBySubcmd(t *testing.T, responses map[string]string) {
	t.Helper()
	stubDockerRun(t, func(args ...string) (string, error) {
		if out, ok := responses[args[0]]; ok {
			return out, nil
		}
		return "", nil
	})
}

// stubDockerRunCapture replaces dockerRun with a stub that records calls.
// failSubcmds lists "verb sub" pairs (e.g. "volume inspect") that should fail.
func stubDockerRunCapture(t *testing.T, failSubcmds ...string) func() []dockerCall {
	t.Helper()
	var calls []dockerCall
	failing := make(map[string]bool, len(failSubcmds))
	for _, s := range failSubcmds {
		failing[s] = true
	}

	stubDockerRun(t, func(args ...string) (string, error) {
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

// stubRunInteractive replaces runInteractive with a mock that records the
// args of the most recent call.
func stubRunInteractive(t *testing.T) func() []string {
	t.Helper()
	var capturedArgs []string

	orig := runInteractive
	runInteractive = func(args ...string) error {
		capturedArgs = args
		return nil
	}
	t.Cleanup(func() { runInteractive = orig })

	return func() []string { return capturedArgs }
}

// stubRunInteractiveCapturingDockerfile replaces runInteractive with a mock that
// records the rendered Dockerfile content for each "build" call, read from the
// path passed via --file= before buildFromContent removes its temp dir.
func stubRunInteractiveCapturingDockerfile(t *testing.T) func() []string {
	t.Helper()
	var contents []string

	orig := runInteractive
	runInteractive = func(args ...string) error {
		for _, a := range args {
			path, ok := strings.CutPrefix(a, "--file=")
			if !ok {
				continue
			}
			content, err := os.ReadFile(path)
			require.NoError(t, err)
			contents = append(contents, string(content))
		}
		return nil
	}
	t.Cleanup(func() { runInteractive = orig })

	return func() []string { return contents }
}

// stubRunInteractiveAll replaces runInteractive with a mock that records every call.
func stubRunInteractiveAll(t *testing.T) func() [][]string {
	t.Helper()
	var calls [][]string

	orig := runInteractive
	runInteractive = func(args ...string) error {
		cp := make([]string, len(args))
		copy(cp, args)
		calls = append(calls, cp)
		return nil
	}
	t.Cleanup(func() { runInteractive = orig })

	return func() [][]string { return calls }
}

// stubIsTerminal replaces isTerminal with a stub that returns val for the duration of the test.
func stubIsTerminal(t *testing.T, val bool) {
	t.Helper()
	orig := isTerminal
	isTerminal = func() bool { return val }
	t.Cleanup(func() { isTerminal = orig })
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
