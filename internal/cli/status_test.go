package cli

import (
	"bytes"
	"fmt"
	"testing"
	"text/tabwriter"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStatusCmd(t *testing.T) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	docker.SetContext("")
	t.Cleanup(func() { docker.SetContext("") })
	return cmd, &buf
}

func TestRunStatus(t *testing.T) {
	t.Run("daemon not running reports status without listing containers", func(t *testing.T) {
		// Arrange
		cmd, buf := newTestStatusCmd(t)
		stubCheckDockerDaemon(t, func() error { return docker.ErrDaemonNotRunning })
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) {
			t.Fatal("listRunningContainers should not be called when the daemon is down")
			return nil, nil
		})

		// Act
		err := runStatus(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "Docker: not running\n", buf.String())
	})

	t.Run("daemon running with no containers prints zero count", func(t *testing.T) {
		// Arrange
		cmd, buf := newTestStatusCmd(t)
		stubCheckDockerDaemon(t, func() error { return nil })
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) { return nil, nil })

		// Act
		err := runStatus(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "Docker:              running\nContainers running:  0\n", buf.String())
	})

	t.Run("daemon running with containers prints table", func(t *testing.T) {
		// Arrange
		cmd, buf := newTestStatusCmd(t)
		stubCheckDockerDaemon(t, func() error { return nil })
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) {
			return []*docker.ContainerInfo{
				{Name: "agentic-claude-ab12", Image: "agentic-claude", Namespace: "agentic", Tool: "claude", Status: "Up 5 minutes"},
			}, nil
		})

		// Act
		err := runStatus(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Docker:              running\n")
		assert.Contains(t, buf.String(), "Containers running:  1\n")
		assert.Contains(t, buf.String(), "agentic-claude-ab12")
	})

	t.Run("listRunningContainers error propagates", func(t *testing.T) {
		// Arrange
		cmd, _ := newTestStatusCmd(t)
		stubCheckDockerDaemon(t, func() error { return nil })
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) {
			return nil, fmt.Errorf("docker error")
		})

		// Act
		err := runStatus(cmd, nil)

		// Assert
		require.Error(t, err)
	})

	t.Run("docker context set prints header before daemon status", func(t *testing.T) {
		// Arrange
		cmd, buf := newTestStatusCmd(t)
		docker.SetContext("prod")
		stubCheckDockerDaemon(t, func() error { return nil })
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) { return nil, nil })

		// Act
		err := runStatus(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "Docker context:      prod\nDocker:              running\nContainers running:  0\n", buf.String())
	})

	t.Run("docker context unset omits header", func(t *testing.T) {
		// Arrange
		cmd, buf := newTestStatusCmd(t)
		stubCheckDockerDaemon(t, func() error { return nil })
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) { return nil, nil })

		// Act
		err := runStatus(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, buf.String(), "Docker context")
	})
}

func TestWriteContainerStatus(t *testing.T) {
	t.Run("no containers prints nothing", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)

		// Act
		err := writeContainerStatus(w, nil)

		// Assert
		require.NoError(t, err)
		require.NoError(t, w.Flush())
		assert.Equal(t, "", buf.String())
	})

	t.Run("containers print table with blanks dashed", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
		containers := []*docker.ContainerInfo{
			{Name: "agentic-claude-ab12", Image: "agentic-claude", Namespace: "agentic", Tool: "claude", Status: "Up 5 minutes"},
			{Name: "mystery", Image: "not-an-agentic-image", Namespace: "", Tool: "", Status: "Up 1 minute"},
		}

		// Act
		err := writeContainerStatus(w, containers)

		// Assert
		require.NoError(t, err)
		require.NoError(t, w.Flush())
		out := buf.String()
		assert.Contains(t, out, "NAME")
		assert.Contains(t, out, "agentic-claude-ab12")
		assert.Contains(t, out, "mystery")
		assert.Contains(t, out, "-") // dashed blank namespace/tool for the unrecognized image
	})
}
