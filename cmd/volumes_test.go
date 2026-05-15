package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stubCreateVolume(t *testing.T, fn func(string) error) func() {
	t.Helper()
	orig := createVolume
	createVolume = fn
	return func() { createVolume = orig }
}

func stubListVolumes(t *testing.T, fn func() (string, error)) func() {
	t.Helper()
	orig := listVolumes
	listVolumes = fn
	return func() { listVolumes = orig }
}

func stubListVolumeNames(t *testing.T, fn func() ([]string, error)) func() {
	t.Helper()
	orig := listVolumeNames
	listVolumeNames = fn
	return func() { listVolumeNames = orig }
}

func stubRemoveVolume(t *testing.T, fn func(string) error) func() {
	t.Helper()
	orig := removeVolume
	removeVolume = fn
	return func() { removeVolume = orig }
}

func withVolumeStdin(t *testing.T, input string) func() {
	t.Helper()
	orig := volumesStdin
	volumesStdin = strings.NewReader(input)
	return func() { volumesStdin = orig }
}

// --- create ---
func TestRunVolumeCreate_callsCreateVolume(t *testing.T) {
	// Arrange
	var got string
	restore := stubCreateVolume(t, func(name string) error { got = name; return nil })
	defer restore()

	// Act
	err := runVolumeCreate(volumesCreateCmd, []string{"maven"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "maven", got)
}

func TestRunVolumeCreate_printsCreated(t *testing.T) {
	// Arrange
	restore := stubCreateVolume(t, func(string) error { return nil })
	defer restore()

	// Act
	out := captureStdout(t, func() {
		err := runVolumeCreate(volumesCreateCmd, []string{"maven"})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "=> created: maven")
}

func TestRunVolumeCreate_error_propagates(t *testing.T) {
	// Arrange
	restore := stubCreateVolume(t, func(string) error { return fmt.Errorf("docker daemon not running") })
	defer restore()

	// Act
	err := runVolumeCreate(volumesCreateCmd, []string{"maven"})

	// Assert
	assert.ErrorContains(t, err, "docker daemon not running")
}

// --- list ---
func TestRunVolumeList_printsRawOutput(t *testing.T) {
	// Arrange
	restore := stubListVolumes(t, func() (string, error) {
		return "DRIVER    VOLUME NAME\nlocal     maven\n", nil
	})
	defer restore()

	// Act
	out := captureStdout(t, func() {
		err := runVolumeList(volumesListCmd, nil)
		require.NoError(t, err)
	})

	// Assert
	assert.Equal(t, "DRIVER    VOLUME NAME\nlocal     maven\n", out)
}

func TestRunVolumeList_error_propagates(t *testing.T) {
	// Arrange
	restore := stubListVolumes(t, func() (string, error) {
		return "", fmt.Errorf("docker daemon not running")
	})
	defer restore()

	// Act
	err := runVolumeList(volumesListCmd, nil)

	// Assert
	assert.ErrorContains(t, err, "docker daemon not running")
}

// --- remove (named) ---
func TestRunVolumeRemove_named_callsRemoveVolume(t *testing.T) {
	// Arrange
	var got string
	restore := stubRemoveVolume(t, func(name string) error { got = name; return nil })
	defer restore()

	// Act
	err := runVolumeRemove(volumesRemoveCmd, []string{"maven"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "maven", got)
}

func TestRunVolumeRemove_named_printsDeleted(t *testing.T) {
	// Arrange
	restore := stubRemoveVolume(t, func(string) error { return nil })
	defer restore()

	// Act
	out := captureStdout(t, func() {
		err := runVolumeRemove(volumesRemoveCmd, []string{"maven"})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "=> deleted: maven")
}

func TestRunVolumeRemove_named_error_propagates(t *testing.T) {
	// Arrange
	restore := stubRemoveVolume(t, func(string) error {
		return fmt.Errorf("'maven' is not an agentic-managed volume")
	})
	defer restore()

	// Act
	err := runVolumeRemove(volumesRemoveCmd, []string{"maven"})

	// Assert
	assert.ErrorContains(t, err, "not an agentic-managed volume")
}

// --- remove (no name) ---

func TestRunVolumeRemove_noName_empty_printsMessage(t *testing.T) {
	// Arrange
	restoreNames := stubListVolumeNames(t, func() ([]string, error) { return nil, nil })
	defer restoreNames()
	var removeCalled bool
	restoreRemove := stubRemoveVolume(t, func(string) error { removeCalled = true; return nil })
	defer restoreRemove()

	// Act
	out := captureStdout(t, func() {
		err := runVolumeRemove(volumesRemoveCmd, nil)
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "No agentic-managed volumes found.")
	assert.False(t, removeCalled)
}

func TestRunVolumeRemove_noName_confirmedY_removesAll(t *testing.T) {
	// Arrange
	restoreNames := stubListVolumeNames(t, func() ([]string, error) {
		return []string{"maven", "gradle"}, nil
	})
	defer restoreNames()
	var removed []string
	restoreRemove := stubRemoveVolume(t, func(name string) error { removed = append(removed, name); return nil })
	defer restoreRemove()
	restoreStdin := withVolumeStdin(t, "y\n")
	defer restoreStdin()

	// Act
	err := runVolumeRemove(volumesRemoveCmd, nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"maven", "gradle"}, removed)
}

func TestRunVolumeRemove_noName_confirmedUpperY_removesAll(t *testing.T) {
	// Arrange
	restoreNames := stubListVolumeNames(t, func() ([]string, error) {
		return []string{"maven"}, nil
	})
	defer restoreNames()
	var removed []string
	restoreRemove := stubRemoveVolume(t, func(name string) error { removed = append(removed, name); return nil })
	defer restoreRemove()
	restoreStdin := withVolumeStdin(t, "Y\n")
	defer restoreStdin()

	// Act
	err := runVolumeRemove(volumesRemoveCmd, nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"maven"}, removed)
}

func TestRunVolumeRemove_noName_declinedN_skipsRemoval(t *testing.T) {
	// Arrange
	restoreNames := stubListVolumeNames(t, func() ([]string, error) {
		return []string{"maven"}, nil
	})
	defer restoreNames()
	var removeCalled bool
	restoreRemove := stubRemoveVolume(t, func(string) error { removeCalled = true; return nil })
	defer restoreRemove()
	restoreStdin := withVolumeStdin(t, "n\n")
	defer restoreStdin()

	// Act
	err := runVolumeRemove(volumesRemoveCmd, nil)

	// Assert
	require.NoError(t, err)
	assert.False(t, removeCalled)
}

func TestRunVolumeRemove_noName_emptyInput_skipsRemoval(t *testing.T) {
	// Arrange
	restoreNames := stubListVolumeNames(t, func() ([]string, error) {
		return []string{"maven"}, nil
	})
	defer restoreNames()
	var removeCalled bool
	restoreRemove := stubRemoveVolume(t, func(string) error { removeCalled = true; return nil })
	defer restoreRemove()
	restoreStdin := withVolumeStdin(t, "\n")
	defer restoreStdin()

	// Act
	err := runVolumeRemove(volumesRemoveCmd, nil)

	// Assert
	require.NoError(t, err)
	assert.False(t, removeCalled)
}

func TestRunVolumeRemove_noName_listError_propagates(t *testing.T) {
	// Arrange
	restoreNames := stubListVolumeNames(t, func() ([]string, error) {
		return nil, fmt.Errorf("docker daemon not running")
	})
	defer restoreNames()

	// Act
	err := runVolumeRemove(volumesRemoveCmd, nil)

	// Assert
	assert.ErrorContains(t, err, "docker daemon not running")
}

func TestRunVolumeRemove_noName_removeError_propagates(t *testing.T) {
	// Arrange
	restoreNames := stubListVolumeNames(t, func() ([]string, error) {
		return []string{"maven", "gradle"}, nil
	})
	defer restoreNames()
	var removeCalled int
	restoreRemove := stubRemoveVolume(t, func(string) error {
		removeCalled++
		return fmt.Errorf("remove failed")
	})
	defer restoreRemove()
	restoreStdin := withVolumeStdin(t, "y\n")
	defer restoreStdin()

	// Act
	err := runVolumeRemove(volumesRemoveCmd, nil)

	// Assert
	assert.ErrorContains(t, err, "remove failed")
	assert.Equal(t, 1, removeCalled, "should stop after first error")
}

func TestRunVolumeRemove_noName_printsVolumesBeforePrompt(t *testing.T) {
	// Arrange
	restoreNames := stubListVolumeNames(t, func() ([]string, error) {
		return []string{"maven", "gradle"}, nil
	})
	defer restoreNames()
	restoreRemove := stubRemoveVolume(t, func(string) error { return nil })
	defer restoreRemove()
	restoreStdin := withVolumeStdin(t, "n\n")
	defer restoreStdin()

	// Act
	out := captureStdout(t, func() {
		err := runVolumeRemove(volumesRemoveCmd, nil)
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "Volumes to remove:")
	assert.Contains(t, out, "  maven")
	assert.Contains(t, out, "  gradle")
}
