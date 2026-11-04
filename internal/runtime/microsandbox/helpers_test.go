package microsandbox

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// updateGolden rewrites the golden files instead of comparing against them: go test -run dryRun -update.
var updateGolden = flag.Bool("update", false, "rewrite golden files")

// dryRunOwner matches the host uid/gid in mount options, replaced by normalizeDryRun.
var dryRunOwner = regexp.MustCompile(`uid=[0-9]+,gid=[0-9]+`)

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

// stubDryRunEnv fixes the host env that dry-run output reads: terminal vars and the timezone.
func stubDryRunEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"COLORTERM", "TERM", "NO_COLOR", "FORCE_COLOR"} {
		t.Setenv(key, "")
	}
	t.Setenv("TZ", "Etc/UTC")
}

// normalizeDryRun replaces toolHome and the host uid/gid in out with fixed placeholders.
func normalizeDryRun(out, toolHome string) string {
	out = strings.ReplaceAll(out, toolHome, "<tool-home>")
	return dryRunOwner.ReplaceAllString(out, "uid=<uid>,gid=<gid>")
}

// assertGolden compares got with testdata/name, or rewrites the file with -update.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateGolden {
		require.NoError(t, os.MkdirAll("testdata", 0o750))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(want), got)
}
