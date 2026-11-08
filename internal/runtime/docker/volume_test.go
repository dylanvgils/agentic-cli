package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

func TestEnsureNamedVolumes(t *testing.T) {
	client := newTestClient()

	t.Run("refuses a spec without a host user", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)

		// Act
		err := client.EnsureNamedVolumes(sandbox.RunSpec{Volumes: []string{"maven:/container"}}, "busybox")

		// Assert
		assert.ErrorIs(t, err, errNoHostUser)
		assert.Empty(t, get())
	})

	for _, tc := range []struct {
		name    string
		volumes []string
	}{
		{name: "skips an absolute path", volumes: []string{"/host/path:/container"}},
		{name: "skips the expanded tool home", volumes: []string{"$TOOL_HOME/data:/container"}},
		{name: "skips a Windows absolute path", volumes: []string{`C:\Users\foo:/container`, `c:\data:/container`}},
		{name: "skips an empty left side", volumes: []string{":/container"}},
		{name: "skips an empty list", volumes: []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			get := stubDockerRunCapture(t, client)

			// Act
			err := client.EnsureNamedVolumes(sandbox.RunSpec{User: &testUser, Volumes: tc.volumes, ToolHome: "/home/.agentic"}, "")

			// Assert
			require.NoError(t, err)
			assert.Empty(t, get())
		})
	}

	t.Run("existing volume skips create and chown", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client) // inspect succeeds -> volume exists

		// Act
		err := client.EnsureNamedVolumes(sandbox.RunSpec{User: &testUser, Volumes: []string{"maven:/container"}}, "")

		// Assert
		require.NoError(t, err)
		calls := get()
		require.Len(t, calls, 1)
		assert.Equal(t, []string{"volume", "inspect", "maven"}, calls[0].args)
	})

	t.Run("new volume creates and chowns", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client, "volume inspect")

		// Act
		err := client.EnsureNamedVolumes(sandbox.RunSpec{User: &testUser, Volumes: []string{"maven:/container"}}, "busybox")

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
		assert.Equal(t, []string{"chown", testUser.String(), "/vol"}, calls[2].args[len(calls[2].args)-3:], "owned by the run's user")
	})

	t.Run("create fails returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, client, "volume inspect", "volume create")

		// Act
		err := client.EnsureNamedVolumes(sandbox.RunSpec{User: &testUser, Volumes: []string{"maven:/container"}}, "")

		// Assert
		assert.ErrorContains(t, err, "create volume maven")
	})

	t.Run("chown fails returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, client, "volume inspect", "run")

		// Act
		err := client.EnsureNamedVolumes(sandbox.RunSpec{User: &testUser, Volumes: []string{"maven:/container"}}, "")

		// Assert
		assert.ErrorContains(t, err, "chown volume maven")
	})

	t.Run("multiple volumes", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client, "volume inspect")

		// Act
		err := client.EnsureNamedVolumes(sandbox.RunSpec{User: &testUser, Volumes: []string{
			"/host:/container",
			"maven:/m2",
			"gradle:/gradle",
		}}, "")

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

}

func TestCreateVolume(t *testing.T) {
	client := newTestClient()

	t.Run("calls docker with label", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)

		// Act
		err := client.CreateVolume("maven")

		// Assert
		require.NoError(t, err)
		calls := get()
		require.Len(t, calls, 1)
		assert.Equal(t, []string{"volume", "create", "--label=project=agentic-cli", "maven"}, calls[0].args)
	})

	t.Run("wraps error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, client, "volume create")

		// Act
		err := client.CreateVolume("maven")

		// Assert
		assert.ErrorContains(t, err, "create volume maven")
	})
}

func TestListVolumeNames(t *testing.T) {
	client := newTestClient()

	t.Run("lists the project's volume names", func(t *testing.T) {
		// Arrange
		var capturedArgs []string
		stubDockerRun(t, client, func(args ...string) (string, error) {
			capturedArgs = args
			return "maven\ngradle\n", nil
		})

		// Act
		names, err := client.ListVolumeNames()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"maven", "gradle"}, names)
		assert.Contains(t, capturedArgs, "--quiet")
		assert.Contains(t, capturedArgs, "--filter=label=project=agentic-cli")
	})

	t.Run("empty output returns empty", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", nil)

		// Act
		names, err := client.ListVolumeNames()

		// Assert
		require.NoError(t, err)
		assert.Empty(t, names)
	})

	t.Run("propagates error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, client, "volume ls")

		// Act
		_, err := client.ListVolumeNames()

		// Assert
		assert.Error(t, err)
	})
}

func TestRemoveVolume(t *testing.T) {
	client := newTestClient()

	t.Run("valid label calls rm", func(t *testing.T) {
		// Arrange
		var calls []dockerCall
		stubDockerRun(t, client, func(args ...string) (string, error) {
			calls = append(calls, dockerCall{args: args})
			if args[0] == "volume" && args[1] == "inspect" {
				return "agentic-cli\n", nil
			}
			return "", nil
		})

		// Act
		err := client.RemoveVolume("maven")

		// Assert
		require.NoError(t, err)
		require.Len(t, calls, 2)
		assert.Equal(t, "inspect", calls[0].args[1])
		assert.Equal(t, "rm", calls[1].args[1])
		assert.Equal(t, "maven", calls[1].args[2])
	})

	t.Run("inspect fails returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, client, "volume inspect")

		// Act
		err := client.RemoveVolume("maven")

		// Assert
		assert.ErrorContains(t, err, "not an agentic-managed volume")
	})

	t.Run("wrong label returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "other-project\n", nil)

		// Act
		err := client.RemoveVolume("maven")

		// Assert
		assert.ErrorContains(t, err, "not an agentic-managed volume")
	})

	t.Run("rm fails propagates error", func(t *testing.T) {
		// Arrange
		stubDockerRun(t, client, func(args ...string) (string, error) {
			if args[0] == "volume" && args[1] == "inspect" {
				return "agentic-cli\n", nil
			}
			return "", fmt.Errorf("stub: volume rm failed")
		})

		// Act
		err := client.RemoveVolume("maven")

		// Assert
		assert.ErrorContains(t, err, "volume rm failed")
	})
}
