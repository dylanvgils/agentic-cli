package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func mockDockerRun(t *testing.T, output string, err error) func() {
	t.Helper()
	orig := dockerRun
	dockerRun = func(_ ...string) (string, error) { return output, err }
	return func() { dockerRun = orig }
}

func TestResolveContainerHome_found(t *testing.T) {
	// Arrange
	restore := mockDockerRun(t, `["PATH=/usr/bin","TOOL_HOME=/home/claude"]`, nil)
	defer restore()

	// Act
	result := ResolveContainerHome("agentic-claude")

	// Assert
	assert.Equal(t, "/home/claude", result)
}

func TestResolveContainerHome_firstMatch(t *testing.T) {
	// Arrange
	restore := mockDockerRun(t, `["TOOL_HOME=/home/claude","OTHER=value","TOOL_HOME=/other"]`, nil)
	defer restore()

	// Act
	result := ResolveContainerHome("agentic-claude")

	// Assert
	assert.Equal(t, "/home/claude", result)
}

func TestResolveContainerHome_notPresent(t *testing.T) {
	// Arrange
	restore := mockDockerRun(t, `["PATH=/usr/bin","HOME=/root"]`, nil)
	defer restore()

	// Act
	result := ResolveContainerHome("agentic-claude")

	// Assert
	assert.Equal(t, "/root", result)
}

func TestResolveContainerHome_emptyEnv(t *testing.T) {
	// Arrange
	restore := mockDockerRun(t, `[]`, nil)
	defer restore()

	// Act
	result := ResolveContainerHome("agentic-claude")

	// Assert
	assert.Equal(t, "/root", result)
}

func TestResolveContainerHome_dockerError(t *testing.T) {
	// Arrange
	restore := mockDockerRun(t, "", fmt.Errorf("image not found"))
	defer restore()

	// Act
	result := ResolveContainerHome("agentic-missing")

	// Assert
	assert.Equal(t, "/root", result)
}

func TestResolveContainerHome_malformedJSON(t *testing.T) {
	// Arrange
	restore := mockDockerRun(t, "not json", nil)
	defer restore()

	// Act
	result := ResolveContainerHome("agentic-claude")

	// Assert
	assert.Equal(t, "/root", result)
}
