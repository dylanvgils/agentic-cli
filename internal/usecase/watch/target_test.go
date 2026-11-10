package watch

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
)

func TestTarget_Args(t *testing.T) {
	// Arrange
	target := Target{
		RunID:         testRunID,
		Runtime:       sandbox.RuntimeDocker,
		ToolHome:      "/example.test/home",
		DockerContext: "example",
		PID:           4242,
	}

	// Act
	args := target.Args()

	// Assert
	assert.Equal(t, []string{
		"--run=" + string(testRunID),
		"--runtime=docker",
		"--home=/example.test/home",
		"--docker-context=example",
		"--pid=4242",
	}, args)
}
