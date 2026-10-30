//go:build integration

package platform

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// Docker runs a docker command and fails the test on error.
func Docker(t *testing.T, args ...string) {
	t.Helper()
	out, err := exec.Command("docker", args...).CombinedOutput()
	require.NoError(t, err, string(out))
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
