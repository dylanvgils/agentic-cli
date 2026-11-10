package watch

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
)

func TestTarget_Args(t *testing.T) {
	// Arrange
	target := Target{
		PID:           4242,
		Runtime:       sandbox.RuntimeDocker,
		ToolHome:      "/example.test/home",
		DockerContext: "example",
		RunID:         testRunID,
	}

	// Act
	args := target.Args()

	// Assert
	assert.Equal(t, []string{
		"--pid=4242",
		"--runtime=docker",
		"--home=/example.test/home",
		"--docker-context=example",
		"--run=" + string(testRunID),
	}, args)
}
