package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListRunningContainers(t *testing.T) {
	client := newTestClient()

	t.Run("lists the project's containers and derives namespace/tool from image name", func(t *testing.T) {
		// Arrange
		var calls [][]string
		stubDockerRun(t, client, func(args ...string) (string, error) {
			calls = append(calls, args)
			return `{"Names":"agentic-claude-ab12","Image":"agentic-claude","Status":"Up 5 minutes","Ports":"127.0.0.1:3000->3000/tcp"}` + "\n" +
				`{"Names":"myproject-copilot-cd34","Image":"myproject-copilot","Status":"Up 1 minute"}`, nil
		})

		// Act
		containers, err := client.ListRunningContainers()

		// Assert
		require.NoError(t, err)
		require.Len(t, containers, 2)
		assert.Equal(t, &ContainerInfo{
			Name:      "agentic-claude-ab12",
			Image:     "agentic-claude",
			Namespace: "agentic",
			Tool:      "claude",
			Status:    "Up 5 minutes",
			Ports:     "127.0.0.1:3000->3000/tcp",
		}, containers[0])
		assert.Equal(t, &ContainerInfo{
			Name: "myproject-copilot-cd34", Image: "myproject-copilot",
			Namespace: "myproject", Tool: "copilot", Status: "Up 1 minute",
		}, containers[1])
		assert.Equal(t, [][]string{{"ps", "--format={{json .}}", "--filter=label=project=agentic-cli"}}, calls)
	})

	t.Run("no running containers returns empty slice", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", nil)

		// Act
		containers, err := client.ListRunningContainers()

		// Assert
		require.NoError(t, err)
		assert.Empty(t, containers)
	})

	t.Run("docker error returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", fmt.Errorf("docker daemon not running"))

		// Act
		containers, err := client.ListRunningContainers()

		// Assert
		require.Error(t, err)
		assert.Nil(t, containers)
	})
}
