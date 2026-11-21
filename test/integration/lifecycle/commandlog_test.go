//go:build integration

package lifecycle

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommandLog checks a run records the docker commands it ran in its log dir.
func TestCommandLog(t *testing.T) {
	// Arrange
	before := commandLogs(t)
	tool := platform.NewToolRun(t).
		Allowing("example.test").
		WithProxy()

	// Act
	tool.Run("true")

	// Assert
	created := newPaths(commandLogs(t), before)
	require.Len(t, created, 1, "expected one new command log")
	got, err := os.ReadFile(created[0])
	require.NoError(t, err)
	assert.Contains(t, string(got), `"command":"docker","args":["run",`)
	assert.Contains(t, string(got), `"exit_code":0`)
	assert.Contains(t, string(got), `"reason":"RunTool`)
}
