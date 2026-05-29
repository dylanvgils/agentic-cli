package docker

import (
	"io"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateTool(t *testing.T) {
	origStdin := dockerRunStdin
	dockerRunStdin = func(_ io.Reader, _ ...string) (string, error) { return "", nil }
	t.Cleanup(func() { dockerRunStdin = origStdin })

	t.Run("recovers build from label", func(t *testing.T) {
		// Arrange
		stubDockerRunBySubcmd(t, map[string]string{
			"inspect": `{"Id":"sha256:abcdef","Size":1048576,"Config":{"Labels":{"agentic.base":"node@24.0.0,java@21.0.1"}}}`,
		})
		getCalls := stubRunInteractiveAll(t)

		// Act
		err := UpdateTool("claude", "agentic-claude", tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
		calls := getCalls()
		require.NotEmpty(t, calls)

		buildCall := calls[0]
		noCacheFilter := false
		for _, a := range buildCall {
			if strings.Contains(a, "no-cache-filter") {
				noCacheFilter = true
			}
		}
		assert.True(t, noCacheFilter, "expected --no-cache-filter in build call after label recovery")
	})

	t.Run("respects existing base override", func(t *testing.T) {
		// Arrange
		var inspectCalled bool
		orig := dockerRun
		dockerRun = func(args ...string) (string, error) {
			if args[0] == "inspect" {
				inspectCalled = true
			}
			return "", nil
		}
		defer func() { dockerRun = orig }()
		stubRunInteractiveAll(t)

		// Act
		err := UpdateTool("claude", "agentic-claude", tools.BuildOptions{BaseOverride: "java"})

		// Assert
		require.NoError(t, err)
		assert.False(t, inspectCalled, "expected InspectImage to be skipped when BaseOverride is already set")
	})

	t.Run("always sets no-cache filter", func(t *testing.T) {
		// Arrange
		stubDockerRunBySubcmd(t, nil)
		getCalls := stubRunInteractiveAll(t)

		// Act — pass NoCache:false to confirm NoCacheTool alone triggers --no-cache-filter on the tool stage
		err := UpdateTool("claude", "agentic-claude", tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
		calls := getCalls()
		require.NotEmpty(t, calls)
		buildCall := calls[0]
		assert.Contains(t, buildCall, "--no-cache-filter=tool", "tool build must skip cache via --no-cache-filter=tool")
	})
}
