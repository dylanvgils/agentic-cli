package cli

import (
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/watch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRunID is a valid run ID.
const testRunID sandbox.RunID = "0123456789ab"

func Test_watchCmd(t *testing.T) {
	// Act
	args := watch.Target{RunID: testRunID}.Args()

	// Assert
	for _, arg := range args {
		name, _, _ := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		defined := watchCmd.Flags().Lookup(name) != nil || watchCmd.InheritedFlags().Lookup(name) != nil
		assert.True(t, defined, "the watchdog's --%s is a flag of the command", name)
	}
}

func Test_watchTarget(t *testing.T) {
	t.Run("reads the target from the flags and the tool home", func(t *testing.T) {
		// Arrange
		stubToolHome(t, "/example.test/home")
		cmd := newTestWatchCmd(t,
			"--run="+string(testRunID),
			"--runtime=docker",
			"--docker-context=example",
			"--pid=4242",
		)

		// Act
		target, err := watchTarget(cmd)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, watch.Target{
			RunID:         testRunID,
			Runtime:       sandbox.RuntimeDocker,
			ToolHome:      "/example.test/home",
			DockerContext: "example",
			PID:           4242,
		}, target)
	})

	t.Run("refuses an invalid run id", func(t *testing.T) {
		// Arrange
		cmd := newTestWatchCmd(t, "--run=../x", "--pid=4242")

		// Act
		_, err := watchTarget(cmd)

		// Assert
		require.EqualError(t, err, `invalid run id "../x"`)
	})
}
