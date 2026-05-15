package docker

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dockerCall struct {
	args []string
}

// captureDockerCmd replaces dockerCmd with a stub that records calls.
// failSubcmds lists "verb sub" pairs (e.g. "volume inspect") that should fail.
func captureDockerCmd(t *testing.T, failSubcmds ...string) (func() []dockerCall, func()) {
	t.Helper()
	var calls []dockerCall
	failing := make(map[string]bool, len(failSubcmds))
	for _, s := range failSubcmds {
		failing[s] = true
	}

	orig := dockerCmd
	dockerCmd = func(args ...string) *exec.Cmd {
		calls = append(calls, dockerCall{args: args})
		key := args[0]
		if len(args) > 1 {
			key += " " + args[1]
		}
		if failing[key] {
			return exec.Command("false")
		}
		return exec.Command("true")
	}

	get := func() []dockerCall { return calls }
	restore := func() { dockerCmd = orig }
	return get, restore
}

func TestEnsureNamedVolumes_skipsAbsolutePath(t *testing.T) {
	get, restore := captureDockerCmd(t)
	defer restore()

	err := EnsureNamedVolumes([]string{"/host/path:/container"}, "", "")

	require.NoError(t, err)
	assert.Empty(t, get())
}

func TestEnsureNamedVolumes_skipsToolHomeExpanded(t *testing.T) {
	get, restore := captureDockerCmd(t)
	defer restore()

	err := EnsureNamedVolumes([]string{"$TOOL_HOME/data:/container"}, "/home/.agentic", "")

	require.NoError(t, err)
	assert.Empty(t, get())
}

func TestEnsureNamedVolumes_skipsEmptyLeft(t *testing.T) {
	get, restore := captureDockerCmd(t)
	defer restore()

	err := EnsureNamedVolumes([]string{":/container"}, "", "")

	require.NoError(t, err)
	assert.Empty(t, get())
}

func TestEnsureNamedVolumes_existingVolume_skipsCreateAndChown(t *testing.T) {
	get, restore := captureDockerCmd(t) // inspect succeeds -> volume exists
	defer restore()

	err := EnsureNamedVolumes([]string{"maven:/container"}, "", "")

	require.NoError(t, err)
	calls := get()
	require.Len(t, calls, 1)
	assert.Equal(t, []string{"volume", "inspect", "maven"}, calls[0].args)
}

func TestEnsureNamedVolumes_newVolume_createsAndChowns(t *testing.T) {
	get, restore := captureDockerCmd(t, "volume inspect")
	defer restore()

	err := EnsureNamedVolumes([]string{"maven:/container"}, "", "")

	require.NoError(t, err)
	calls := get()
	require.Len(t, calls, 3)
	assert.Equal(t, []string{"volume", "inspect", "maven"}, calls[0].args)
	assert.Equal(t, []string{"volume", "create", "--label", "project=agentic-cli", "maven"}, calls[1].args)
	assert.Equal(t, "run", calls[2].args[0])
	assert.Contains(t, calls[2].args, "maven:/vol")
	assert.Contains(t, calls[2].args, "busybox")
	assert.Contains(t, calls[2].args, "chown")
}

func TestEnsureNamedVolumes_createFails_returnsError(t *testing.T) {
	_, restore := captureDockerCmd(t, "volume inspect", "volume create")
	defer restore()

	err := EnsureNamedVolumes([]string{"maven:/container"}, "", "")

	assert.ErrorContains(t, err, "create volume maven")
}

func TestEnsureNamedVolumes_chownFails_returnsError(t *testing.T) {
	orig := dockerCmd
	dockerCmd = func(args ...string) *exec.Cmd {
		if args[0] == "volume" && args[1] == "inspect" {
			return exec.Command("false")
		}
		if args[0] == "run" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	defer func() { dockerCmd = orig }()

	err := EnsureNamedVolumes([]string{"maven:/container"}, "", "")

	assert.ErrorContains(t, err, "chown volume maven")
}

func TestEnsureNamedVolumes_multipleVolumes(t *testing.T) {
	get, restore := captureDockerCmd(t, "volume inspect")
	defer restore()

	err := EnsureNamedVolumes([]string{
		"/host:/container",
		"maven:/m2",
		"gradle:/gradle",
	}, "", "")

	require.NoError(t, err)
	calls := get()
	// Two named volumes: inspect+create+chown each = 6 calls
	assert.Len(t, calls, 6)
	var inspects []string
	for _, c := range calls {
		if c.args[0] == "volume" && c.args[1] == "inspect" {
			inspects = append(inspects, c.args[2])
		}
	}
	assert.Equal(t, []string{"maven", "gradle"}, inspects)
}

func TestEnsureNamedVolumes_emptyList(t *testing.T) {
	get, restore := captureDockerCmd(t)
	defer restore()

	err := EnsureNamedVolumes([]string{}, "", "")

	require.NoError(t, err)
	assert.Empty(t, get())
}
