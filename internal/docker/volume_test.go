package docker

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dockerCall struct {
	args []string
}

// captureDockerRun replaces dockerRun with a stub that records calls.
// failSubcmds lists "verb sub" pairs (e.g. "volume inspect") that should fail.
func captureDockerRun(t *testing.T, failSubcmds ...string) (func() []dockerCall, func()) {
	t.Helper()
	var calls []dockerCall
	failing := make(map[string]bool, len(failSubcmds))
	for _, s := range failSubcmds {
		failing[s] = true
	}

	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		calls = append(calls, dockerCall{args: args})
		key := args[0]
		if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
			key += " " + args[1]
		}
		if failing[key] {
			return "", fmt.Errorf("stub: %s failed", key)
		}
		return "", nil
	}

	get := func() []dockerCall { return calls }
	restore := func() { dockerRun = orig }
	return get, restore
}

func TestEnsureNamedVolumes_skipsAbsolutePath(t *testing.T) {
	// Arrange
	get, restore := captureDockerRun(t)
	defer restore()

	// Act
	err := EnsureNamedVolumes([]string{"/host/path:/container"}, "", "")

	// Assert
	require.NoError(t, err)
	assert.Empty(t, get())
}

func TestEnsureNamedVolumes_skipsToolHomeExpanded(t *testing.T) {
	// Arrange
	get, restore := captureDockerRun(t)
	defer restore()

	// Act
	err := EnsureNamedVolumes([]string{"$TOOL_HOME/data:/container"}, "/home/.agentic", "")

	// Assert
	require.NoError(t, err)
	assert.Empty(t, get())
}

func TestEnsureNamedVolumes_skipsWindowsAbsolutePath(t *testing.T) {
	// Arrange
	get, restore := captureDockerRun(t)
	defer restore()

	// Act
	err := EnsureNamedVolumes([]string{`C:\Users\foo:/container`}, "", "")

	// Assert
	require.NoError(t, err)
	assert.Empty(t, get())
}

func TestEnsureNamedVolumes_skipsWindowsAbsolutePathLowercase(t *testing.T) {
	// Arrange
	get, restore := captureDockerRun(t)
	defer restore()

	// Act
	err := EnsureNamedVolumes([]string{`c:\data:/container`}, "", "")

	// Assert
	require.NoError(t, err)
	assert.Empty(t, get())
}

func TestEnsureNamedVolumes_skipsEmptyLeft(t *testing.T) {
	// Arrange
	get, restore := captureDockerRun(t)
	defer restore()

	// Act
	err := EnsureNamedVolumes([]string{":/container"}, "", "")

	// Assert
	require.NoError(t, err)
	assert.Empty(t, get())
}

func TestEnsureNamedVolumes_existingVolume_skipsCreateAndChown(t *testing.T) {
	// Arrange
	get, restore := captureDockerRun(t) // inspect succeeds -> volume exists
	defer restore()

	// Act
	err := EnsureNamedVolumes([]string{"maven:/container"}, "", "")

	// Assert
	require.NoError(t, err)
	calls := get()
	require.Len(t, calls, 1)
	assert.Equal(t, []string{"volume", "inspect", "maven"}, calls[0].args)
}

func TestEnsureNamedVolumes_newVolume_createsAndChowns(t *testing.T) {
	// Arrange
	get, restore := captureDockerRun(t, "volume inspect")
	defer restore()

	// Act
	err := EnsureNamedVolumes([]string{"maven:/container"}, "", "")

	// Assert
	require.NoError(t, err)
	calls := get()
	require.Len(t, calls, 3)
	assert.Equal(t, []string{"volume", "inspect", "maven"}, calls[0].args)
	assert.Equal(t, []string{"volume", "create", "--label=project=agentic-cli", "maven"}, calls[1].args)
	assert.Equal(t, "run", calls[2].args[0])
	assert.Contains(t, calls[2].args, "--volume=maven:/vol")
	assert.Contains(t, calls[2].args, "--user=root")
	assert.Contains(t, calls[2].args, "busybox")
	assert.Contains(t, calls[2].args, "chown")
}

func TestEnsureNamedVolumes_createFails_returnsError(t *testing.T) {
	// Arrange
	_, restore := captureDockerRun(t, "volume inspect", "volume create")
	defer restore()

	// Act
	err := EnsureNamedVolumes([]string{"maven:/container"}, "", "")

	// Assert
	assert.ErrorContains(t, err, "create volume maven")
}

func TestEnsureNamedVolumes_chownFails_returnsError(t *testing.T) {
	// Arrange
	_, restore := captureDockerRun(t, "volume inspect", "run")
	defer restore()

	// Act
	err := EnsureNamedVolumes([]string{"maven:/container"}, "", "")

	// Assert
	assert.ErrorContains(t, err, "chown volume maven")
}

func TestEnsureNamedVolumes_multipleVolumes(t *testing.T) {
	// Arrange
	get, restore := captureDockerRun(t, "volume inspect")
	defer restore()

	// Act
	err := EnsureNamedVolumes([]string{
		"/host:/container",
		"maven:/m2",
		"gradle:/gradle",
	}, "", "")

	// Assert
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
	// Arrange
	get, restore := captureDockerRun(t)
	defer restore()

	// Act
	err := EnsureNamedVolumes([]string{}, "", "")

	// Assert
	require.NoError(t, err)
	assert.Empty(t, get())
}

// --- CreateVolume ---

func TestCreateVolume_callsDockerWithLabel(t *testing.T) {
	// Arrange
	get, restore := captureDockerRun(t)
	defer restore()

	// Act
	err := CreateVolume("maven")

	// Assert
	require.NoError(t, err)
	calls := get()
	require.Len(t, calls, 1)
	assert.Equal(t, []string{"volume", "create", "--label=project=agentic-cli", "maven"}, calls[0].args)
}

func TestCreateVolume_wrapsError(t *testing.T) {
	// Arrange
	_, restore := captureDockerRun(t, "volume create")
	defer restore()

	// Act
	err := CreateVolume("maven")

	// Assert
	assert.ErrorContains(t, err, "create volume maven")
}

// --- ListVolumes ---

func TestListVolumes_callsWithFilter(t *testing.T) {
	// Arrange
	get, restore := captureDockerRun(t)
	defer restore()

	// Act
	_, err := ListVolumes()

	// Assert
	require.NoError(t, err)
	calls := get()
	require.Len(t, calls, 1)
	assert.Equal(t, []string{"volume", "ls", "--filter=label=project=agentic-cli"}, calls[0].args)
}

func TestListVolumes_returnsOutput(t *testing.T) {
	// Arrange
	orig := dockerRun
	dockerRun = func(_ ...string) (string, error) { return "DRIVER    VOLUME NAME\nlocal     maven\n", nil }
	defer func() { dockerRun = orig }()

	// Act
	out, err := ListVolumes()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "DRIVER    VOLUME NAME\nlocal     maven\n", out)
}

func TestListVolumes_propagatesError(t *testing.T) {
	// Arrange
	_, restore := captureDockerRun(t, "volume ls")
	defer restore()

	// Act
	_, err := ListVolumes()

	// Assert
	assert.Error(t, err)
}

// --- ListVolumeNames ---

func TestListVolumeNames_callsDockerWithQuietAndFilter(t *testing.T) {
	// Arrange
	get, restore := captureDockerRun(t)
	defer restore()

	// Act
	_, err := ListVolumeNames()

	// Assert
	require.NoError(t, err)
	calls := get()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].args, "--quiet")
	assert.Contains(t, calls[0].args, "--filter=label=project=agentic-cli")
}

func TestListVolumeNames_splitsLines(t *testing.T) {
	// Arrange
	orig := dockerRun
	dockerRun = func(_ ...string) (string, error) { return "maven\ngradle\n", nil }
	defer func() { dockerRun = orig }()

	// Act
	names, err := ListVolumeNames()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"maven", "gradle"}, names)
}

func TestListVolumeNames_emptyOutput_returnsEmpty(t *testing.T) {
	// Arrange
	orig := dockerRun
	dockerRun = func(_ ...string) (string, error) { return "", nil }
	defer func() { dockerRun = orig }()

	// Act
	names, err := ListVolumeNames()

	// Assert
	require.NoError(t, err)
	assert.Empty(t, names)
}

func TestListVolumeNames_propagatesError(t *testing.T) {
	// Arrange
	_, restore := captureDockerRun(t, "volume ls")
	defer restore()

	// Act
	_, err := ListVolumeNames()

	// Assert
	assert.Error(t, err)
}

// --- RemoveVolume ---

func TestRemoveVolume_validLabel_callsRm(t *testing.T) {
	// Arrange
	var calls []dockerCall
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		calls = append(calls, dockerCall{args: args})
		if args[0] == "volume" && args[1] == "inspect" {
			return "agentic-cli\n", nil
		}
		return "", nil
	}
	defer func() { dockerRun = orig }()

	// Act
	err := RemoveVolume("maven")

	// Assert
	require.NoError(t, err)
	require.Len(t, calls, 2)
	assert.Equal(t, "inspect", calls[0].args[1])
	assert.Equal(t, "rm", calls[1].args[1])
	assert.Equal(t, "maven", calls[1].args[2])
}

func TestRemoveVolume_inspectFails_returnsError(t *testing.T) {
	// Arrange
	_, restore := captureDockerRun(t, "volume inspect")
	defer restore()

	// Act
	err := RemoveVolume("maven")

	// Assert
	assert.ErrorContains(t, err, "not an agentic-managed volume")
}

func TestRemoveVolume_wrongLabel_returnsError(t *testing.T) {
	// Arrange
	orig := dockerRun
	dockerRun = func(_ ...string) (string, error) { return "other-project\n", nil }
	defer func() { dockerRun = orig }()

	// Act
	err := RemoveVolume("maven")

	// Assert
	assert.ErrorContains(t, err, "not an agentic-managed volume")
}

func TestRemoveVolume_rmFails_propagatesError(t *testing.T) {
	// Arrange
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		if args[0] == "volume" && args[1] == "inspect" {
			return "agentic-cli\n", nil
		}
		return "", fmt.Errorf("stub: volume rm failed")
	}
	defer func() { dockerRun = orig }()

	// Act
	err := RemoveVolume("maven")

	// Assert
	assert.ErrorContains(t, err, "volume rm failed")
}
