package watch

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
)

func TestTarget_Args(t *testing.T) {
	// Arrange
	target := Target{
		RunRef: sandbox.RunRef{
			ID:       testRunID,
			Runtime:  sandbox.RuntimeDocker,
			Tool:     "claude",
			ToolHome: "/example.test/home",
		},
		DockerContext: "example",
		PID:           4242,
	}

	// Act
	args := target.Args()

	// Assert
	assert.Equal(t, []string{
		"--run=" + string(testRunID),
		"--runtime=docker",
		"--tool=claude",
		"--home=/example.test/home",
		"--docker-context=example",
		"--pid=4242",
	}, args)
}
