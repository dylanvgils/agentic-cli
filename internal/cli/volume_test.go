package cli

import (
	"cmp"
	"errors"
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dylanvgils/agentic-cli/internal/usecase/volumes"
)

func Test_runVolumeCreate(t *testing.T) {
	t.Run("creates the volume and says so", func(t *testing.T) {
		// Arrange
		var got string
		stubCreateVolume(t, func(name string) error { got = name; return nil })
		logBuf := iostreamstest.StubErr(t)

		// Act
		err := runVolumeCreate(volumeCreateCmd, []string{"maven"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "maven", got)
		assert.Equal(t, "agentic: created volume maven\n", logBuf.String())
	})

	t.Run("error propagates", func(t *testing.T) {
		// Arrange
		stubCreateVolume(t, func(string) error { return fmt.Errorf("docker daemon not running") })

		// Act
		err := runVolumeCreate(volumeCreateCmd, []string{"maven"})

		// Assert
		assert.ErrorContains(t, err, "docker daemon not running")
	})
}

func Test_runVolumeList(t *testing.T) {
	stubMicrosandbox(t, &fakeMicrosandbox{})

	t.Run("lists the volumes in a table", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) { return []string{"maven", "npm"}, nil })
		stubMicrosandbox(t, &fakeMicrosandbox{listVolumeNames: func() ([]string, error) { return []string{"gradle"}, nil }})
		var err error

		// Act
		logs := iostreamstest.CaptureOut(t, func() { err = runVolumeList(volumeListCmd, nil) })

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "NAME    RUNTIME\nmaven   docker\nnpm     docker\ngradle  microsandbox\n", logs)
	})

	t.Run("no volumes says so on stderr", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) { return nil, nil })
		errLog := iostreamstest.StubErr(t)
		var err error

		// Act
		logs := iostreamstest.CaptureOut(t, func() { err = runVolumeList(volumeListCmd, nil) })

		// Assert
		require.NoError(t, err)
		assert.Empty(t, logs)
		assert.Equal(t, "agentic: no agentic-managed volumes found\n", errLog.String())
	})

	t.Run("error propagates", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) {
			return nil, fmt.Errorf("docker daemon not running")
		})

		// Act
		err := runVolumeList(volumeListCmd, nil)

		// Assert
		assert.ErrorContains(t, err, "docker daemon not running")
	})
}

func Test_runVolumeRemove(t *testing.T) {
	stubMicrosandbox(t, &fakeMicrosandbox{})

	t.Run("named removes the volume and says so", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) { return []string{"maven"}, nil })
		var got string
		stubRemoveVolume(t, func(name string) error { got = name; return nil })
		logBuf := iostreamstest.StubErr(t)

		// Act
		err := runVolumeRemove(volumeRemoveCmd, []string{"maven"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "maven", got)
		assert.Equal(t, "agentic: removed volume maven\n", logBuf.String())
	})

	t.Run("named error propagates", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) { return nil, nil })

		// Act
		err := runVolumeRemove(volumeRemoveCmd, []string{"maven"})

		// Assert
		assert.ErrorContains(t, err, "not an agentic-managed volume")
	})

	t.Run("no name without volumes says so", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) { return nil, nil })
		logBuf := iostreamstest.StubErr(t)

		// Act
		err := runVolumeRemove(volumeRemoveCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic: no agentic-managed volumes found\n", logBuf.String())
	})

	t.Run("no name confirmed y removes every runtime's volumes", func(t *testing.T) {
		// Arrange
		logBuf := iostreamstest.StubErr(t)
		stubListVolumeNames(t, func() ([]string, error) { return []string{"maven"}, nil })
		var removed []string
		stubRemoveVolume(t, func(name string) error { removed = append(removed, "docker "+name); return nil })
		stubMicrosandbox(t, &fakeMicrosandbox{
			listVolumeNames: func() ([]string, error) { return []string{"gradle"}, nil },
			removeVolume:    func(name string) error { removed = append(removed, "msb "+name); return nil },
		})
		iostreamstest.StubIn(t, "y\n")

		// Act
		err := runVolumeRemove(volumeRemoveCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"docker maven", "msb gradle"}, removed)
		assert.Contains(t, logBuf.String(), "   gradle (microsandbox)\n")
		assert.Contains(t, logBuf.String(), "agentic: removing 2 volume(s)")
	})

	t.Run("no name declined lists the volumes before the prompt and removes nothing", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) { return []string{"gradle", "maven"}, nil })
		var removeCalled bool
		stubRemoveVolume(t, func(string) error { removeCalled = true; return nil })
		iostreamstest.StubIn(t, "n\n")
		logBuf := iostreamstest.StubErr(t)

		// Act
		err := runVolumeRemove(volumeRemoveCmd, nil)

		// Assert - list and prompt share stderr, list first
		require.NoError(t, err)
		assert.False(t, removeCalled)
		assert.Equal(t, "agentic: volumes to remove:\n   gradle\n   maven\nagentic: remove all agentic-managed volumes? [y/N] agentic: nothing removed\n", logBuf.String())
	})

	for _, tc := range []struct {
		name      string
		listErr   error
		removeErr error
	}{
		{name: "list", listErr: fmt.Errorf("docker daemon not running")},
		{name: "remove", removeErr: fmt.Errorf("remove failed")},
	} {
		t.Run("no name "+tc.name+" error propagates", func(t *testing.T) {
			// Arrange
			stubListVolumeNames(t, func() ([]string, error) { return []string{"maven"}, tc.listErr })
			stubRemoveVolume(t, func(string) error { return tc.removeErr })
			iostreamstest.StubIn(t, "y\n")
			iostreamstest.StubErr(t)

			// Act
			err := runVolumeRemove(volumeRemoveCmd, nil)

			// Assert
			assert.ErrorIs(t, err, cmp.Or(tc.listErr, tc.removeErr))
		})
	}
}

func Test_newVolumeService(t *testing.T) {
	t.Run("includes microsandbox when it's installed", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) { return nil, nil })
		stubMicrosandbox(t, &fakeMicrosandbox{listVolumeNames: func() ([]string, error) { return []string{"gradle"}, nil }})

		// Act
		all, err := newVolumeService().List()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []volumes.Volume{{Name: "gradle", Runtime: sandbox.RuntimeMicrosandbox}}, all)
	})

	t.Run("leaves microsandbox out when it isn't installed", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) { return nil, nil })
		stubMicrosandbox(t, &fakeMicrosandbox{
			available:       func() error { return errors.New("msb not found on PATH") },
			listVolumeNames: func() ([]string, error) { return []string{"gradle"}, nil },
		})

		// Act
		all, err := newVolumeService().List()

		// Assert
		require.NoError(t, err)
		assert.Empty(t, all)
	})
}
