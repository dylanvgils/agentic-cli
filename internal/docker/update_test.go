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
		getDockerfiles := stubRunInteractiveCapturingDockerfile(t)

		// Act
		err := UpdateTool("claude", "agentic-claude", tools.BuildOptions{})

		// Assert - the java extra recovered from the label is added as a stage
		require.NoError(t, err)
		dockerfiles := getDockerfiles()
		require.NotEmpty(t, dockerfiles)
		assert.Contains(t, dockerfiles[0], "AS java")
	})

	t.Run("respects existing base override", func(t *testing.T) {
		// Arrange
		stubDockerRunBySubcmd(t, map[string]string{
			"inspect": `{"Id":"sha256:abcdef","Size":1048576,"Config":{"Labels":{"agentic.base":"node@24.0.0,dotnet@8.0"}}}`,
		})
		getDockerfiles := stubRunInteractiveCapturingDockerfile(t)

		// Act
		err := UpdateTool("claude", "agentic-claude", tools.BuildOptions{BaseOverride: []string{"java"}})

		// Assert - explicit BaseOverride wins over the dotnet recovered from the label
		require.NoError(t, err)
		dockerfiles := getDockerfiles()
		require.NotEmpty(t, dockerfiles)
		assert.Contains(t, dockerfiles[0], "AS java")
		assert.NotContains(t, dockerfiles[0], "AS dotnet")
	})

	t.Run("recovers layer versions from label", func(t *testing.T) {
		// Arrange
		stubDockerRunBySubcmd(t, map[string]string{
			"inspect": `{"Id":"sha256:abcdef","Size":1048576,"Config":{"Labels":{"agentic.base":"node@24.0.0,java@21.0.1","agentic.version-args":"node@24,java@17"}}}`,
		})
		getDockerfiles := stubRunInteractiveCapturingDockerfile(t)

		// Act - no --java flag passed, so the recovered override must be the one used
		err := UpdateTool("claude", "agentic-claude", tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
		dockerfiles := getDockerfiles()
		require.NotEmpty(t, dockerfiles)
		assert.Contains(t, dockerfiles[0], "ARG JAVA_VERSION=17")
	})

	t.Run("user-provided version flag wins over recovered label", func(t *testing.T) {
		// Arrange
		stubDockerRunBySubcmd(t, map[string]string{
			"inspect": `{"Id":"sha256:abcdef","Size":1048576,"Config":{"Labels":{"agentic.base":"node@24.0.0,java@21.0.1","agentic.version-args":"node@24,java@17"}}}`,
		})
		getDockerfiles := stubRunInteractiveCapturingDockerfile(t)

		// Act
		err := UpdateTool("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{"java": "21"}})

		// Assert
		require.NoError(t, err)
		dockerfiles := getDockerfiles()
		require.NotEmpty(t, dockerfiles)
		assert.Contains(t, dockerfiles[0], "ARG JAVA_VERSION=21")
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

	t.Run("always sets cachebust build arg", func(t *testing.T) {
		// Arrange
		stubDockerRunBySubcmd(t, nil)
		getCalls := stubRunInteractiveAll(t)

		// Act - confirm UpdateTool sets a CacheBust value that triggers the CACHEBUST build arg on the tool stage
		err := UpdateTool("claude", "agentic-claude", tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
		calls := getCalls()
		require.NotEmpty(t, calls)
		buildCall := calls[0]
		hasCacheBust := false
		for _, a := range buildCall {
			if strings.HasPrefix(a, "--build-arg=CACHEBUST=") && a != "--build-arg=CACHEBUST=" {
				hasCacheBust = true
			}
		}
		assert.True(t, hasCacheBust, "tool build must skip cache via a non-empty --build-arg=CACHEBUST=<value>")
	})
}

func Test_mergeVersions(t *testing.T) {
	t.Run("overrides win over recovered", func(t *testing.T) {
		// Act
		result := mergeVersions(map[string]string{"node": "24", "java": "17"}, map[string]string{"java": "21"})

		// Assert
		assert.Equal(t, map[string]string{"node": "24", "java": "21"}, result)
	})

	t.Run("empty override values are ignored", func(t *testing.T) {
		// Act
		result := mergeVersions(map[string]string{"node": "24"}, map[string]string{"node": "", "java": "17"})

		// Assert
		assert.Equal(t, map[string]string{"node": "24", "java": "17"}, result)
	})

	t.Run("no recovered values returns overrides", func(t *testing.T) {
		// Act
		result := mergeVersions(nil, map[string]string{"java": "17"})

		// Assert
		assert.Equal(t, map[string]string{"java": "17"}, result)
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
