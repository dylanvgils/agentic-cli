package cli

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunVolumeCreate(t *testing.T) {
	t.Run("calls create volume", func(t *testing.T) {
		// Arrange
		var got string
		stubCreateVolume(t, func(name string) error { got = name; return nil })

		// Act
		err := runVolumeCreate(volumesCreateCmd, []string{"maven"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "maven", got)
	})

	t.Run("prints created", func(t *testing.T) {
		// Arrange
		stubCreateVolume(t, func(string) error { return nil })
		logBuf := stubErrLog(t)

		// Act
		err := runVolumeCreate(volumesCreateCmd, []string{"maven"})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logBuf.String(), "agentic: created volume maven")
	})

	t.Run("error propagates", func(t *testing.T) {
		// Arrange
		stubCreateVolume(t, func(string) error { return fmt.Errorf("docker daemon not running") })

		// Act
		err := runVolumeCreate(volumesCreateCmd, []string{"maven"})

		// Assert
		assert.ErrorContains(t, err, "docker daemon not running")
	})
}

func TestRunVolumeList(t *testing.T) {
	t.Run("prints raw output", func(t *testing.T) {
		// Arrange
		stubListVolumes(t, func() (string, error) {
			return "DRIVER    VOLUME NAME\nlocal     maven\n", nil
		})

		// Act
		out := captureStdout(t, func() {
			err := runVolumeList(volumesListCmd, nil)
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, "DRIVER    VOLUME NAME\nlocal     maven\n", out)
	})

	t.Run("error propagates", func(t *testing.T) {
		// Arrange
		stubListVolumes(t, func() (string, error) {
			return "", fmt.Errorf("docker daemon not running")
		})

		// Act
		err := runVolumeList(volumesListCmd, nil)

		// Assert
		assert.ErrorContains(t, err, "docker daemon not running")
	})
}

func TestRunVolumeRemove(t *testing.T) {
	t.Run("named calls remove volume", func(t *testing.T) {
		// Arrange
		var got string
		stubRemoveVolume(t, func(name string) error { got = name; return nil })

		// Act
		err := runVolumeRemove(volumesRemoveCmd, []string{"maven"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "maven", got)
	})

	t.Run("named prints deleted", func(t *testing.T) {
		// Arrange
		stubRemoveVolume(t, func(string) error { return nil })
		logBuf := stubErrLog(t)

		// Act
		err := runVolumeRemove(volumesRemoveCmd, []string{"maven"})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logBuf.String(), "agentic: removed volume maven")
	})

	t.Run("named error propagates", func(t *testing.T) {
		// Arrange
		stubRemoveVolume(t, func(string) error {
			return fmt.Errorf("'maven' is not an agentic-managed volume")
		})

		// Act
		err := runVolumeRemove(volumesRemoveCmd, []string{"maven"})

		// Assert
		assert.ErrorContains(t, err, "not an agentic-managed volume")
	})

	t.Run("no name empty prints message", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) { return nil, nil })
		var removeCalled bool
		stubRemoveVolume(t, func(string) error { removeCalled = true; return nil })
		logBuf := stubErrLog(t)

		// Act
		err := runVolumeRemove(volumesRemoveCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logBuf.String(), "agentic: no agentic-managed volumes found")
		assert.False(t, removeCalled)
	})

	t.Run("no name confirmed y removes all", func(t *testing.T) {
		// Arrange
		logBuf := stubErrLog(t)
		stubListVolumeNames(t, func() ([]string, error) {
			return []string{"maven", "gradle"}, nil
		})
		var removed []string
		stubRemoveVolume(t, func(name string) error { removed = append(removed, name); return nil })
		stubStdin(t, "y\n")

		// Act
		err := runVolumeRemove(volumesRemoveCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"maven", "gradle"}, removed)
		assert.Contains(t, logBuf.String(), "agentic: removing 2 volume(s)")
	})

	t.Run("no name confirmed upper y removes all", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) {
			return []string{"maven"}, nil
		})
		var removed []string
		stubRemoveVolume(t, func(name string) error { removed = append(removed, name); return nil })
		stubStdin(t, "Y\n")

		// Act
		err := runVolumeRemove(volumesRemoveCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"maven"}, removed)
	})

	t.Run("no name declined n skips removal", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) {
			return []string{"maven"}, nil
		})
		var removeCalled bool
		stubRemoveVolume(t, func(string) error { removeCalled = true; return nil })
		stubStdin(t, "n\n")

		// Act
		err := runVolumeRemove(volumesRemoveCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.False(t, removeCalled)
	})

	t.Run("no name empty input skips removal", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) {
			return []string{"maven"}, nil
		})
		var removeCalled bool
		stubRemoveVolume(t, func(string) error { removeCalled = true; return nil })
		stubStdin(t, "\n")

		// Act
		err := runVolumeRemove(volumesRemoveCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.False(t, removeCalled)
	})

	t.Run("no name list error propagates", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) {
			return nil, fmt.Errorf("docker daemon not running")
		})

		// Act
		err := runVolumeRemove(volumesRemoveCmd, nil)

		// Assert
		assert.ErrorContains(t, err, "docker daemon not running")
	})

	t.Run("no name remove error propagates", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) {
			return []string{"maven", "gradle"}, nil
		})
		var removeCalled int
		stubRemoveVolume(t, func(string) error {
			removeCalled++
			return fmt.Errorf("remove failed")
		})
		stubStdin(t, "y\n")

		// Act
		err := runVolumeRemove(volumesRemoveCmd, nil)

		// Assert
		assert.ErrorContains(t, err, "remove failed")
		assert.Equal(t, 1, removeCalled, "should stop after first error")
	})

	t.Run("no name prints volumes before prompt", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) {
			return []string{"maven", "gradle"}, nil
		})
		stubRemoveVolume(t, func(string) error { return nil })
		stubStdin(t, "n\n")

		// Act
		out := captureStdout(t, func() {
			err := runVolumeRemove(volumesRemoveCmd, nil)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "Volumes to remove:")
		assert.Contains(t, out, "  maven")
		assert.Contains(t, out, "  gradle")
	})
}
