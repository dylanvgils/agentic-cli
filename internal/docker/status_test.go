package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListRunningContainers(t *testing.T) {
	t.Run("parses containers and derives namespace/tool from image name", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t,
			`{"Names":"agentic-claude-ab12","Image":"agentic-claude","Status":"Up 5 minutes"}`+"\n"+
				`{"Names":"myproject-copilot-cd34","Image":"myproject-copilot","Status":"Up 1 minute"}`,
			nil)

		// Act
		containers, err := ListRunningContainers()

		// Assert
		require.NoError(t, err)
		require.Len(t, containers, 2)
		assert.Equal(t, &ContainerInfo{
			Name: "agentic-claude-ab12", Image: "agentic-claude",
			Namespace: "agentic", Tool: "claude", Status: "Up 5 minutes",
		}, containers[0])
		assert.Equal(t, &ContainerInfo{
			Name: "myproject-copilot-cd34", Image: "myproject-copilot",
			Namespace: "myproject", Tool: "copilot", Status: "Up 1 minute",
		}, containers[1])
	})

	t.Run("unrecognized image leaves namespace and tool blank", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, `{"Names":"mystery","Image":"not-an-agentic-image","Status":"Up 2 minutes"}`, nil)

		// Act
		containers, err := ListRunningContainers()

		// Assert
		require.NoError(t, err)
		require.Len(t, containers, 1)
		assert.Equal(t, "", containers[0].Namespace)
		assert.Equal(t, "", containers[0].Tool)
	})

	t.Run("no running containers returns empty slice", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", nil)

		// Act
		containers, err := ListRunningContainers()

		// Assert
		require.NoError(t, err)
		assert.Empty(t, containers)
	})

	t.Run("docker error returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", fmt.Errorf("docker daemon not running"))

		// Act
		containers, err := ListRunningContainers()

		// Assert
		require.Error(t, err)
		assert.Nil(t, containers)
	})

	t.Run("filters on the agentic project label", func(t *testing.T) {
		// Arrange
		var calls [][]string
		stubDockerRun(t, func(args ...string) (string, error) {
			calls = append(calls, args)
			return "", nil
		})

		// Act
		_, err := ListRunningContainers()

		// Assert
		require.NoError(t, err)
		require.Len(t, calls, 1)
		assert.Equal(t, []string{"ps", "--format={{json .}}", "--filter=label=project=agentic-cli"}, calls[0])
	})
}
