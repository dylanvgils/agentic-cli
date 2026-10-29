package sandbox

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsReservedEnvName(t *testing.T) {
	t.Run("config names are reserved in every run", func(t *testing.T) {
		for _, key := range []string{"TOOL_HOME", "CONTAINER_HOME", "AGENTIC_MARKETPLACES", "AGENTIC_PROXY_CA"} {
			// Act
			reserved := IsReservedEnvName(key, false, false, nil)

			// Assert
			assert.True(t, reserved, key)
		}
	})

	t.Run("proxy names are reserved only with the proxy", func(t *testing.T) {
		for _, key := range []string{"HTTP_PROXY", "https_proxy", "NO_PROXY", "NODE_USE_ENV_PROXY"} {
			// Act
			without, with := IsReservedEnvName(key, false, false, nil), IsReservedEnvName(key, true, false, nil)

			// Assert
			assert.False(t, without, key)
			assert.True(t, with, key)
		}
	})

	t.Run("docker client names are reserved only with dind", func(t *testing.T) {
		for _, key := range []string{"DOCKER_HOST", "DOCKER_TLS_VERIFY", "DOCKER_CERT_PATH", "DOCKER_CONTEXT", "DOCKER_CONFIG"} {
			// Act
			without, with := IsReservedEnvName(key, false, false, nil), IsReservedEnvName(key, false, true, nil)

			// Assert
			assert.False(t, without, key)
			assert.True(t, with, key)
		}
	})

	t.Run("integration names are reserved only with their integration", func(t *testing.T) {
		// Act
		without, with := IsReservedEnvName("MAVEN_ARGS", true, false, nil), IsReservedEnvName("MAVEN_ARGS", true, false, []string{"maven"})

		// Assert
		assert.False(t, without)
		assert.True(t, with)
	})

	t.Run("other names are free", func(t *testing.T) {
		for _, key := range []string{"MAVEN_OPTS", "TERM", "DOCKER_BUILDKIT"} {
			// Act
			reserved := IsReservedEnvName(key, true, true, nil)

			// Assert
			assert.False(t, reserved, key)
		}
	})
}

func TestIsPortableEnvName(t *testing.T) {
	t.Run("accepts letters, digits and underscores", func(t *testing.T) {
		for _, name := range []string{"GH_TOKEN", "_x", "a1"} {
			// Act
			portable := IsPortableEnvName(name)

			// Assert
			assert.True(t, portable, name)
		}
	})

	t.Run("refuses other names", func(t *testing.T) {
		for _, name := range []string{"", "1A", "A-B", "A.B", "A B"} {
			// Act
			portable := IsPortableEnvName(name)

			// Assert
			assert.False(t, portable, name)
		}
	})
}

func TestResolveEnv(t *testing.T) {
	// Arrange
	clearTerminalEnv(t)
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TZ", "America/New_York")

	// Act
	env := ResolveEnv([]string{"NO_COLOR=0", "TZ=UTC"})

	// Assert
	assert.Equal(t, []string{"NO_COLOR=1", "TZ=America/New_York", "NO_COLOR=0", "TZ=UTC"}, env, "entries come last so they win")
}

func TestEnvArgs(t *testing.T) {
	// Arrange
	clearTerminalEnv(t)
	t.Setenv("TZ", "UTC")

	// Act
	args := EnvArgs([]string{"MAVEN_OPTS=-Dfoo=bar"})

	// Assert
	assert.Equal(t, []string{"--env=TZ=UTC", "--env=MAVEN_OPTS=-Dfoo=bar"}, args)
}

func Test_terminalEnv(t *testing.T) {
	t.Run("forwards each set terminal var from the host", func(t *testing.T) {
		// Arrange
		t.Setenv("COLORTERM", "truecolor")
		t.Setenv("TERM", "xterm-256color")
		t.Setenv("NO_COLOR", "1")
		t.Setenv("FORCE_COLOR", "1")

		// Act
		env := terminalEnv()

		// Assert
		assert.Equal(t, []string{"COLORTERM=truecolor", "TERM=xterm-256color", "NO_COLOR=1", "FORCE_COLOR=1"}, env)
	})

	t.Run("skips unset terminal vars", func(t *testing.T) {
		// Arrange
		clearTerminalEnv(t)
		t.Setenv("TERM", "xterm-256color")

		// Act
		env := terminalEnv()

		// Assert
		assert.Equal(t, []string{"TERM=xterm-256color"}, env)
	})
}

func Test_timezoneEnv(t *testing.T) {
	t.Run("forwards the host timezone", func(t *testing.T) {
		// Act
		env := timezoneEnv("Europe/Amsterdam")

		// Assert
		assert.Equal(t, []string{"TZ=Europe/Amsterdam"}, env)
	})

	t.Run("empty when the timezone can't be detected", func(t *testing.T) {
		// Act
		env := timezoneEnv("")

		// Assert
		assert.Empty(t, env)
	})
}

func Test_entryEnv(t *testing.T) {
	// Arrange
	t.Setenv("CI", "true")
	os.Unsetenv("AGENTIC_NONEXISTENT_VAR") //nolint:errcheck

	// Act
	env := entryEnv([]string{"MAVEN_OPTS=-Dfoo=bar", "CI", "AGENTIC_NONEXISTENT_VAR"})

	// Assert
	assert.Equal(t, []string{"MAVEN_OPTS=-Dfoo=bar", "CI"}, env, "a set bare key stays bare, so its value stays out of argv")
}
