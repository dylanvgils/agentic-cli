package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveContainerHome(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, `["PATH=/usr/bin","TOOL_HOME=/home/claude"]`, nil)

		// Act
		result := ResolveContainerHome("agentic-claude")

		// Assert
		assert.Equal(t, "/home/claude", result)
	})

	t.Run("first match", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, `["TOOL_HOME=/home/claude","OTHER=value","TOOL_HOME=/other"]`, nil)

		// Act
		result := ResolveContainerHome("agentic-claude")

		// Assert
		assert.Equal(t, "/home/claude", result)
	})

	t.Run("not present", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, `["PATH=/usr/bin","HOME=/root"]`, nil)

		// Act
		result := ResolveContainerHome("agentic-claude")

		// Assert
		assert.Equal(t, "/root", result)
	})

	t.Run("empty env", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, `[]`, nil)

		// Act
		result := ResolveContainerHome("agentic-claude")

		// Assert
		assert.Equal(t, "/root", result)
	})

	t.Run("docker error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", fmt.Errorf("image not found"))

		// Act
		result := ResolveContainerHome("agentic-missing")

		// Assert
		assert.Equal(t, "/root", result)
	})

	t.Run("malformed JSON", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "not json", nil)

		// Act
		result := ResolveContainerHome("agentic-claude")

		// Assert
		assert.Equal(t, "/root", result)
	})
}
