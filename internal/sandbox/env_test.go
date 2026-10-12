package sandbox

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveEnv(t *testing.T) {
	stubHostTimezone(t, "")

	t.Run("empty when no terminal vars, timezone or entries", func(t *testing.T) {
		// Arrange
		clearTerminalEnv(t)

		// Act
		env := ResolveEnv(nil)

		// Assert
		assert.Empty(t, env)
	})

	t.Run("forwards each set terminal var from the host", func(t *testing.T) {
		// Arrange
		t.Setenv("COLORTERM", "truecolor")
		t.Setenv("TERM", "xterm-256color")
		t.Setenv("NO_COLOR", "1")
		t.Setenv("FORCE_COLOR", "1")

		// Act
		env := ResolveEnv(nil)

		// Assert
		assert.Equal(t, []string{"COLORTERM=truecolor", "TERM=xterm-256color", "NO_COLOR=1", "FORCE_COLOR=1"}, env)
	})

	t.Run("forwards the host timezone after the terminal vars", func(t *testing.T) {
		// Arrange
		clearTerminalEnv(t)
		t.Setenv("TERM", "xterm-256color")
		stubHostTimezone(t, "Europe/Amsterdam")

		// Act
		env := ResolveEnv(nil)

		// Assert
		assert.Equal(t, []string{"TERM=xterm-256color", "TZ=Europe/Amsterdam"}, env)
	})

	t.Run("passes KEY=VALUE and forwards bare keys set on the host", func(t *testing.T) {
		// Arrange
		clearTerminalEnv(t)
		t.Setenv("CI", "true")
		os.Unsetenv("AGENTIC_NONEXISTENT_VAR") //nolint:errcheck

		// Act
		env := ResolveEnv([]string{"MAVEN_OPTS=-Dfoo=bar", "CI", "AGENTIC_NONEXISTENT_VAR"})

		// Assert
		assert.Equal(t, []string{"MAVEN_OPTS=-Dfoo=bar", "CI=true"}, env)
	})

	t.Run("puts entries after the forwarded vars so they win", func(t *testing.T) {
		// Arrange
		clearTerminalEnv(t)
		t.Setenv("NO_COLOR", "1")
		stubHostTimezone(t, "America/New_York")

		// Act
		env := ResolveEnv([]string{"NO_COLOR=0", "TZ=UTC"})

		// Assert
		assert.Equal(t, []string{"NO_COLOR=1", "TZ=America/New_York", "NO_COLOR=0", "TZ=UTC"}, env)
	})
}
