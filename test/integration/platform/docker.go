//go:build integration

package platform

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Docker runs a docker command, fails the test on error and returns its trimmed stdout.
func Docker(t *testing.T, args ...string) string {
	t.Helper()
	out, err := dockerRun(args...)
	require.NoError(t, err)
	return out
}

// DockerOK reports whether a docker command succeeds.
func DockerOK(args ...string) bool {
	_, err := dockerRun(args...)
	return err == nil
}

// TempDirIn creates a dir under parent, removed on cleanup; use it over t.TempDir for anything bind-mounted.
func TempDirIn(t *testing.T, parent string) string {
	t.Helper()
	dir, err := os.MkdirTemp(parent, "itest-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// UniqueName returns a docker resource name that won't clash with real agentic resources.
func UniqueName(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 6)
	_, err := rand.Read(buf)
	require.NoError(t, err)
	return Namespace + "-" + hex.EncodeToString(buf)
}

// dockerRun runs a docker command and returns its trimmed stdout; the error carries stderr.
func dockerRun(args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("docker", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}
