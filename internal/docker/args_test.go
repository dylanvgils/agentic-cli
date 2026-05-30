package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestForwardEnvArg(t *testing.T) {
	t.Run("empty when key not set", func(t *testing.T) {
		// Arrange
		t.Setenv("TERM", "")

		// Act
		args := forwardEnvArg("TERM")

		// Assert
		assert.Empty(t, args)
	})

	t.Run("builds env flag when key is set", func(t *testing.T) {
		// Arrange
		t.Setenv("TERM", "xterm-256color")

		// Act
		args := forwardEnvArg("TERM")

		// Assert
		assert.Equal(t, []string{"--env=TERM=xterm-256color"}, args)
	})

	t.Run("only includes keys that are set", func(t *testing.T) {
		// Arrange
		t.Setenv("COLORTERM", "truecolor")
		t.Setenv("TERM", "")
		t.Setenv("NO_COLOR", "1")

		// Act
		args := forwardEnvArg("COLORTERM", "TERM", "NO_COLOR")

		// Assert
		assert.Equal(t, []string{"--env=COLORTERM=truecolor", "--env=NO_COLOR=1"}, args)
	})
}

func TestArg(t *testing.T) {
	t.Run("builds flag with value", func(t *testing.T) {
		// Act
		result := arg("volume", "/host:/container")

		// Assert
		assert.Equal(t, "--volume=/host:/container", result)
	})

	t.Run("builds flag without value", func(t *testing.T) {
		// Act
		result := arg("force")

		// Assert
		assert.Equal(t, "--force", result)
	})

	t.Run("empty name panics", func(t *testing.T) {
		// Act + Assert
		assert.Panics(t, func() { arg("") })
	})

	t.Run("dash prefix panics", func(t *testing.T) {
		// Act + Assert
		assert.Panics(t, func() { arg("-flag") })
	})

	t.Run("multiple values panics", func(t *testing.T) {
		// Act + Assert
		assert.Panics(t, func() { arg("filter", "a", "b") })
	})
}
