package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/require"
)

// dockerCall records a single dockerRun invocation.
type dockerCall struct {
	args []string
}

// stubDocker writes a shell script named "docker" to a temp dir and prepends it to PATH.
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

// stubDockerRunCapture replaces dockerRun with a stub that records calls and fails any
// "verb sub" pair (e.g. "volume inspect") listed in failSubcmds.
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

// stubRunInteractive replaces runInteractive with a mock that records the args of the most recent call.
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

// stubRunInteractiveCapturingDockerfile replaces runInteractive with a mock recording the
// rendered Dockerfile content for each "build" call, read from the --file= path before removal.
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

// stubRunInteractiveCapture captures the last runInteractive call's args and, if present, its
// --file Dockerfile content (read inside the stub before the temp file is removed).
func stubRunInteractiveCapture(t *testing.T) func() (args []string, dockerfile string) {
	t.Helper()
	var capturedArgs []string
	var content string

	orig := runInteractive
	runInteractive = func(a ...string) error {
		capturedArgs = a
		for _, x := range a {
			if p, ok := strings.CutPrefix(x, "--file="); ok {
				b, err := os.ReadFile(p)
				require.NoError(t, err)
				content = string(b)
			}
		}
		return nil
	}
	t.Cleanup(func() { runInteractive = orig })

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

// stubDindProbe stubs dockerRun so the readiness probe always fails and `inspect` reports running as the given state.
func stubDindProbe(t *testing.T, running string) func() []dockerCall {
	t.Helper()
	var calls []dockerCall
	stubDockerRun(t, func(args ...string) (string, error) {
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

// stubRunInteractiveError replaces runInteractive with a stub that always returns err.
func stubRunInteractiveError(t *testing.T, err error) {
	t.Helper()
	orig := runInteractive
	runInteractive = func(...string) error { return err }
	t.Cleanup(func() { runInteractive = orig })
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

// stubOwnedResources stubs dockerRun so `ps` and `network ls` return the given rows, recording every call.
func stubOwnedResources(t *testing.T, containers, networks string) func() []dockerCall {
	t.Helper()
	var calls []dockerCall
	stubDockerRun(t, func(args ...string) (string, error) {
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

// ownedRow formats a docker list row as ownedFormat renders it.
func ownedRow(name, owner string, age time.Duration) string {
	return name + "\t" + owner + "\t" + formatLabelTime(time.Now().Add(-age)) + "\n"
}
