//go:build integration

package microsandbox

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// writableScript prints <label>=rw when path can be created (and removes it again), <label>=ro otherwise.
func writableScript(label, path string) string {
	return "touch " + path + " 2>/dev/null && rm " + path + " && echo " + label + "=rw || echo " + label + "=ro"
}

// reachableScript prints <label>=ok when an HTTPS request to host succeeds, <label>=blocked otherwise.
func reachableScript(label, host string) string {
	return "curl --fail --silent --show-error --max-time 20 --output /dev/null https://" + host + " && echo " + label + "=ok || echo " + label + "=blocked"
}

// valueAfter returns the integer that follows prefix on its line in out.
func valueAfter(t *testing.T, out, prefix string) int {
	t.Helper()
	_, rest, ok := strings.Cut(out, prefix)
	require.True(t, ok, "no %q in output", prefix)
	line, _, _ := strings.Cut(rest, "\n")
	value, err := strconv.Atoi(strings.TrimSpace(line))
	require.NoError(t, err)
	return value
}
