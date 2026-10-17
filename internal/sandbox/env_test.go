package sandbox

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	assert.Equal(t, []string{"MAVEN_OPTS=-Dfoo=bar", "CI=true"}, env)
}
