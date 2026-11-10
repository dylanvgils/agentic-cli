//go:build integration

package microsandbox

import (
	"encoding/json"
	"os/exec"
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

// runningVMs returns the names of the running VMs of run id.
func runningVMs(t *testing.T, id string) []string {
	t.Helper()
	out, err := exec.Command("msb", "ls", "--running", "--label", "agentic.run="+id, "--format", "json").Output()
	require.NoError(t, err)

	var vms []struct {
		Name string `json:"name"`
	}
	if len(strings.TrimSpace(string(out))) > 0 {
		require.NoError(t, json.Unmarshal(out, &vms))
	}

	names := make([]string, 0, len(vms))
	for _, vm := range vms {
		names = append(names, vm.Name)
	}
	return names
}
