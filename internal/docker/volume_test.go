package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureNamedVolumes(t *testing.T) {
	t.Run("skips absolute path", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)

		// Act
		err := EnsureNamedVolumes([]string{"/host/path:/container"}, "", "")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, get())
	})

	t.Run("skips tool home expanded", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)

		// Act
		err := EnsureNamedVolumes([]string{"$TOOL_HOME/data:/container"}, "/home/.agentic", "")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, get())
	})

	t.Run("skips Windows absolute path", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)

		// Act
		err := EnsureNamedVolumes([]string{`C:\Users\foo:/container`}, "", "")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, get())
	})

	t.Run("skips Windows absolute path lowercase", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)

		// Act
		err := EnsureNamedVolumes([]string{`c:\data:/container`}, "", "")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, get())
	})

	t.Run("skips empty left", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)

		// Act
		err := EnsureNamedVolumes([]string{":/container"}, "", "")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, get())
	})

	t.Run("existing volume skips create and chown", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t) // inspect succeeds -> volume exists

		// Act
		err := EnsureNamedVolumes([]string{"maven:/container"}, "", "")

		// Assert
		require.NoError(t, err)
		calls := get()
		require.Len(t, calls, 1)
		assert.Equal(t, []string{"volume", "inspect", "maven"}, calls[0].args)
	})

	t.Run("new volume creates and chowns", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, "volume inspect")

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
	})

	t.Run("create fails returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, "volume inspect", "volume create")

		// Act
		err := EnsureNamedVolumes([]string{"maven:/container"}, "", "")

		// Assert
		assert.ErrorContains(t, err, "create volume maven")
	})

	t.Run("chown fails returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, "volume inspect", "run")

		// Act
		err := EnsureNamedVolumes([]string{"maven:/container"}, "", "")

		// Assert
		assert.ErrorContains(t, err, "chown volume maven")
	})

	t.Run("multiple volumes", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, "volume inspect")

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
	})

	t.Run("empty list", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)

		// Act
		err := EnsureNamedVolumes([]string{}, "", "")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, get())
	})
}

func TestCreateVolume(t *testing.T) {
	t.Run("calls docker with label", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)

		// Act
		err := CreateVolume("maven")

		// Assert
		require.NoError(t, err)
		calls := get()
		require.Len(t, calls, 1)
		assert.Equal(t, []string{"volume", "create", "--label=project=agentic-cli", "maven"}, calls[0].args)
	})

	t.Run("wraps error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, "volume create")

		// Act
		err := CreateVolume("maven")

		// Assert
		assert.ErrorContains(t, err, "create volume maven")
	})
}

func TestListVolumes(t *testing.T) {
	t.Run("calls with filter", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)

		// Act
		_, err := ListVolumes()

		// Assert
		require.NoError(t, err)
		calls := get()
		require.Len(t, calls, 1)
		assert.Equal(t, []string{"volume", "ls", "--filter=label=project=agentic-cli"}, calls[0].args)
	})

	t.Run("returns output", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "DRIVER    VOLUME NAME\nlocal     maven\n", nil)

		// Act
		out, err := ListVolumes()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "DRIVER    VOLUME NAME\nlocal     maven\n", out)
	})

	t.Run("propagates error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, "volume ls")

		// Act
		_, err := ListVolumes()

		// Assert
		assert.Error(t, err)
	})
}

func TestListVolumeNames(t *testing.T) {
	t.Run("calls docker with quiet and filter", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)

		// Act
		_, err := ListVolumeNames()

		// Assert
		require.NoError(t, err)
		calls := get()
		require.Len(t, calls, 1)
		assert.Contains(t, calls[0].args, "--quiet")
		assert.Contains(t, calls[0].args, "--filter=label=project=agentic-cli")
	})

	t.Run("splits lines", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "maven\ngradle\n", nil)

		// Act
		names, err := ListVolumeNames()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"maven", "gradle"}, names)
	})

	t.Run("empty output returns empty", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", nil)

		// Act
		names, err := ListVolumeNames()

		// Assert
		require.NoError(t, err)
		assert.Empty(t, names)
	})

	t.Run("propagates error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, "volume ls")

		// Act
		_, err := ListVolumeNames()

		// Assert
		assert.Error(t, err)
	})
}

func TestRemoveVolume(t *testing.T) {
	t.Run("valid label calls rm", func(t *testing.T) {
		// Arrange
		var calls []dockerCall
		stubDockerRun(t, func(args ...string) (string, error) {
			calls = append(calls, dockerCall{args: args})
			if args[0] == "volume" && args[1] == "inspect" {
				return "agentic-cli\n", nil
			}
			return "", nil
		})

		// Act
		err := RemoveVolume("maven")

		// Assert
		require.NoError(t, err)
		require.Len(t, calls, 2)
		assert.Equal(t, "inspect", calls[0].args[1])
		assert.Equal(t, "rm", calls[1].args[1])
		assert.Equal(t, "maven", calls[1].args[2])
	})

	t.Run("inspect fails returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, "volume inspect")

		// Act
		err := RemoveVolume("maven")

		// Assert
		assert.ErrorContains(t, err, "not an agentic-managed volume")
	})

	t.Run("wrong label returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "other-project\n", nil)

		// Act
		err := RemoveVolume("maven")

		// Assert
		assert.ErrorContains(t, err, "not an agentic-managed volume")
	})

	t.Run("rm fails propagates error", func(t *testing.T) {
		// Arrange
		stubDockerRun(t, func(args ...string) (string, error) {
			if args[0] == "volume" && args[1] == "inspect" {
				return "agentic-cli\n", nil
			}
			return "", fmt.Errorf("stub: volume rm failed")
		})

		// Act
		err := RemoveVolume("maven")

		// Assert
		assert.ErrorContains(t, err, "volume rm failed")
	})
}
