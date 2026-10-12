package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestPrintDryRun(t *testing.T) {
	// Arrange
	logs := stubLogs(t)

	// Act
	var err error
	out := captureStdout(t, func() { err = PrintDryRun("tool container", "docker", []string{"run", "--rm"}) })

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "docker run \\\n  --rm\n", out)
	assert.Equal(t, "agentic: tool container:\n\n", logs.String())
}

func Test_shellCommand(t *testing.T) {
	t.Run("puts each arg after the subcommand on its own line", func(t *testing.T) {
		// Act
		cmd := shellCommand("docker", []string{"run", "--env=A=b c", "--env=B=$X", "it's"})

		// Assert
		assert.Equal(t, "docker run \\\n  '--env=A=b c' \\\n  '--env=B=$X' \\\n  'it'\\''s'", cmd)
	})

	t.Run("keeps a lone subcommand on one line", func(t *testing.T) {
		// Act
		cmd := shellCommand("docker", []string{"run"})

		// Assert
		assert.Equal(t, "docker run", cmd)
	})
}
