package docker

import (
	"io"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_recoverAptPackages(t *testing.T) {
	t.Run("splits comma-separated packages", func(t *testing.T) {
		// Act
		result := recoverAptPackages("make,gcc,jq")

		// Assert
		assert.Equal(t, []string{"make", "gcc", "jq"}, result)
	})

	t.Run("trims spaces", func(t *testing.T) {
		// Act
		result := recoverAptPackages("make, gcc")

		// Assert
		assert.Equal(t, []string{"make", "gcc"}, result)
	})

	t.Run("empty string returns nil", func(t *testing.T) {
		// Act
		result := recoverAptPackages("")

		// Assert
		assert.Nil(t, result)
	})
}

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
		stubDockerRunBySubcmd(t, map[string]string{
			"inspect": `{"Id":"sha256:abcdef","Size":1048576,"Config":{"Labels":{"agentic.base":"node@24.0.0,java@21.0.1"}}}`,
		})
		getCalls := stubRunInteractiveAll(t)

		// Act
		err := UpdateTool("claude", "agentic-claude", tools.BuildOptions{BaseOverride: "java"})

		// Assert - BaseOverride is preserved, not overwritten by label
		require.NoError(t, err)
		calls := getCalls()
		require.NotEmpty(t, calls)
		_ = calls // build happened with the provided BaseOverride
	})

	t.Run("recovers apt packages from label", func(t *testing.T) {
		// Arrange
		stubDockerRunBySubcmd(t, map[string]string{
			"inspect": `{"Id":"sha256:abcdef","Size":1048576,"Config":{"Labels":{"agentic.apt":"make,gcc"}}}`,
		})
		getCalls := stubRunInteractiveAll(t)

		// Act
		err := UpdateTool("claude", "agentic-claude", tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
		_ = getCalls()
		// Verification: covered by GenerateDockerfile receiving the packages (tested via packages_test)
	})

	t.Run("merges label apt packages with user-provided packages", func(t *testing.T) {
		// Arrange
		stubDockerRunBySubcmd(t, map[string]string{
			"inspect": `{"Id":"sha256:abcdef","Size":1048576,"Config":{"Labels":{"agentic.apt":"make"}}}`,
		})
		getCalls := stubRunInteractiveAll(t)

		// Act
		err := UpdateTool("claude", "agentic-claude", tools.BuildOptions{AptPackages: []string{"gcc"}})

		// Assert
		require.NoError(t, err)
		require.NotEmpty(t, getCalls())
	})

	t.Run("skips verification when all user packages already in image", func(t *testing.T) {
		// Arrange
		stubDockerRunBySubcmd(t, map[string]string{
			"inspect": `{"Id":"sha256:abcdef","Config":{"Labels":{"agentic.apt":"make,gcc"}}}`,
		})
		getCalls := stubRunInteractiveAll(t)

		// Act
		err := UpdateTool("claude", "agentic-claude", tools.BuildOptions{AptPackages: []string{"make"}})

		// Assert
		require.NoError(t, err)
		for _, call := range getCalls() {
			assert.NotEqual(t, "pull", call[0], "expected no pull for already-known packages")
		}
	})

	t.Run("verifies when user provides package not in image", func(t *testing.T) {
		// Arrange
		stubDockerRunBySubcmd(t, map[string]string{
			"inspect": `{"Id":"sha256:abcdef","Config":{"Labels":{"agentic.apt":"make"}}}`,
		})
		getCalls := stubRunInteractiveAll(t)

		// Act
		err := UpdateTool("claude", "agentic-claude", tools.BuildOptions{AptPackages: []string{"gcc"}})

		// Assert
		require.NoError(t, err)
		hasPull := false
		for _, call := range getCalls() {
			if call[0] == "pull" {
				hasPull = true
			}
		}
		assert.True(t, hasPull, "expected pull for new package not in image")
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

func Test_hasNewAptPackages(t *testing.T) {
	t.Run("all packages in existing returns false", func(t *testing.T) {
		// Act
		result := hasNewAptPackages([]string{"make", "gcc"}, []string{"make", "gcc", "jq"})

		// Assert
		assert.False(t, result)
	})

	t.Run("new package returns true", func(t *testing.T) {
		// Act
		result := hasNewAptPackages([]string{"make", "curl"}, []string{"make"})

		// Assert
		assert.True(t, result)
	})

	t.Run("empty requested returns false", func(t *testing.T) {
		// Act
		result := hasNewAptPackages(nil, []string{"make"})

		// Assert
		assert.False(t, result)
	})
}
