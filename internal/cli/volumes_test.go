package cli

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runVolumeCreate(t *testing.T) {
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

func Test_runVolumeList(t *testing.T) {
	stubMicrosandbox(t, &fakeMicrosandbox{})

	t.Run("lists the volumes grouped by runtime", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) { return []string{"maven", "npm"}, nil })
		stubMicrosandbox(t, &fakeMicrosandbox{listVolumeNames: func() ([]string, error) { return []string{"gradle"}, nil }})
		var err error

		// Act
		logs := captureLog(t, func() { err = runVolumeList(volumesListCmd, nil) })

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "=> docker\n   maven\n   npm\n=> microsandbox\n   gradle\n", logs)
	})

	t.Run("error propagates", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) {
			return nil, fmt.Errorf("docker daemon not running")
		})

		// Act
		err := runVolumeList(volumesListCmd, nil)

		// Assert
		assert.ErrorContains(t, err, "docker daemon not running")
	})
}

func Test_runVolumeRemove(t *testing.T) {
	stubMicrosandbox(t, &fakeMicrosandbox{})

	t.Run("named calls remove volume", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) { return []string{"maven"}, nil })
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
		stubListVolumeNames(t, func() ([]string, error) { return []string{"maven"}, nil })
		stubRemoveVolume(t, func(string) error { return nil })
		logBuf := stubErrLog(t)

		// Act
		err := runVolumeRemove(volumesRemoveCmd, []string{"maven"})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logBuf.String(), "agentic: removed volume maven")
	})

	t.Run("named removes an msb volume", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) { return nil, nil })
		var got string
		stubMicrosandbox(t, &fakeMicrosandbox{
			listVolumeNames: func() ([]string, error) { return []string{"maven"}, nil },
			removeVolume:    func(name string) error { got = name; return nil },
		})

		// Act
		err := runVolumeRemove(volumesRemoveCmd, []string{"maven"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "maven", got)
	})

	t.Run("named error propagates", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) { return nil, nil })

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
			return []string{"gradle", "maven"}, nil
		})
		var removed []string
		stubRemoveVolume(t, func(name string) error { removed = append(removed, name); return nil })
		stubStdin(t, "y\n")

		// Act
		err := runVolumeRemove(volumesRemoveCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"gradle", "maven"}, removed)
		assert.Contains(t, logBuf.String(), "agentic: removing 2 volume(s)")
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
			return []string{"gradle", "maven"}, nil
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

	t.Run("no name lists volumes right before the prompt", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) {
			return []string{"gradle", "maven"}, nil
		})
		stubRemoveVolume(t, func(string) error { return nil })
		stubStdin(t, "n\n")
		logBuf := stubErrLog(t)

		// Act
		err := runVolumeRemove(volumesRemoveCmd, nil)

		// Assert - list and prompt share stderr, list first
		require.NoError(t, err)
		assert.Equal(t, "agentic: volumes to remove:\n   gradle\n   maven\nagentic: remove all agentic-managed volumes? [y/N] ", logBuf.String())
	})

	t.Run("no name removes msb volumes too, marked by runtime", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) { return []string{"maven"}, nil })
		var removed []string
		stubRemoveVolume(t, func(name string) error { removed = append(removed, "docker "+name); return nil })
		stubMicrosandbox(t, &fakeMicrosandbox{
			listVolumeNames: func() ([]string, error) { return []string{"gradle"}, nil },
			removeVolume:    func(name string) error { removed = append(removed, "msb "+name); return nil },
		})
		stubStdin(t, "y\n")
		logBuf := stubErrLog(t)

		// Act
		err := runVolumeRemove(volumesRemoveCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"docker maven", "msb gradle"}, removed)
		assert.Contains(t, logBuf.String(), "   gradle (microsandbox)\n")
	})
}
