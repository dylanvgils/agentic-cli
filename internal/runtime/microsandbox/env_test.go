package microsandbox

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_buildEnvArgs(t *testing.T) {
	t.Setenv("EXAMPLE_TOKEN", "test-secret")

	t.Run("only flags without forwarded vars", func(t *testing.T) {
		// Act
		args, cleanup, err := buildEnvArgs(sandbox.RunSpec{Env: []string{"MAVEN_OPTS=-Xmx1g"}})
		cleanup()

		// Assert
		require.NoError(t, err)
		assert.Contains(t, args, "--env=MAVEN_OPTS=-Xmx1g")
		assert.NotContains(t, args, "--runtime-conf="+dryRunEnvConfPlaceholder)
	})

	t.Run("dry run shows a placeholder and writes nothing", func(t *testing.T) {
		// Act
		args, cleanup, err := buildEnvArgs(sandbox.RunSpec{Env: []string{"EXAMPLE_TOKEN"}, DryRun: true})
		cleanup()

		// Assert
		require.NoError(t, err)
		assert.Contains(t, args, "--runtime-conf="+dryRunEnvConfPlaceholder)
	})
}

func Test_splitEnv(t *testing.T) {
	t.Run("forwards a bare key and keeps pairs as flags", func(t *testing.T) {
		// Act
		flags, forwarded := splitEnv([]string{"MAVEN_OPTS=-Xmx1g", "EXAMPLE_TOKEN"})

		// Assert
		assert.Equal(t, []string{"--env=MAVEN_OPTS=-Xmx1g"}, flags)
		assert.Equal(t, []string{"EXAMPLE_TOKEN"}, forwarded)
	})

	t.Run("a later pair beats an earlier bare key", func(t *testing.T) {
		// Act
		flags, forwarded := splitEnv([]string{"TZ", "TZ=UTC"})

		// Assert
		assert.Equal(t, []string{"--env=TZ=UTC"}, flags)
		assert.Empty(t, forwarded)
	})

	t.Run("a later bare key beats an earlier pair", func(t *testing.T) {
		// Act
		flags, forwarded := splitEnv([]string{"TZ=UTC", "TZ"})

		// Assert
		assert.Empty(t, flags)
		assert.Equal(t, []string{"TZ"}, forwarded)
	})

	t.Run("resolves a non-portable bare key to a flag", func(t *testing.T) {
		// Arrange
		t.Setenv("EXAMPLE.TOKEN", "test-secret")

		// Act
		flags, forwarded := splitEnv([]string{"EXAMPLE.TOKEN"})

		// Assert
		assert.Equal(t, []string{"--env=EXAMPLE.TOKEN=test-secret"}, flags)
		assert.Empty(t, forwarded)
	})
}

func Test_writeEnvConf(t *testing.T) {
	// Act
	path, err := writeEnvConf([]string{"EXAMPLE_TOKEN", "CI"})

	// Assert
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(path) })
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "env:\n  \"EXAMPLE_TOKEN\": \"${EXAMPLE_TOKEN}\"\n  \"CI\": \"${CI}\"\n", string(data))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
