package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArg(t *testing.T) {
	t.Run("builds flag with value", func(t *testing.T) {
		// Act
		result := Arg("volume", "/host:/container")

		// Assert
		assert.Equal(t, "--volume=/host:/container", result)
	})

	t.Run("keeps an = in the value", func(t *testing.T) {
		// Act
		result := Arg("env", "FOO=bar")

		// Assert
		assert.Equal(t, "--env=FOO=bar", result)
	})

	t.Run("builds flag without value", func(t *testing.T) {
		// Act
		result := Arg("force")

		// Assert
		assert.Equal(t, "--force", result)
	})

	t.Run("empty name panics", func(t *testing.T) {
		// Act + Assert
		assert.Panics(t, func() { Arg("") })
	})

	t.Run("dash prefix panics", func(t *testing.T) {
		// Act + Assert
		assert.Panics(t, func() { Arg("-flag") })
	})

	t.Run("multiple values panics", func(t *testing.T) {
		// Act + Assert
		assert.Panics(t, func() { Arg("filter", "a", "b") })
	})
}

func TestShellCommand(t *testing.T) {
	t.Run("puts each arg after the subcommand on its own line", func(t *testing.T) {
		// Act
		cmd := ShellCommand("docker", []string{"run", "--env=A=b c", "--env=B=$X", "it's"})

		// Assert
		assert.Equal(t, "docker run \\\n  '--env=A=b c' \\\n  '--env=B=$X' \\\n  'it'\\''s'", cmd)
	})

	t.Run("keeps a lone subcommand on one line", func(t *testing.T) {
		// Act
		cmd := ShellCommand("docker", []string{"run"})

		// Assert
		assert.Equal(t, "docker run", cmd)
	})
}
