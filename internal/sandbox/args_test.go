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
	assert.Equal(t, hostShell().command("docker", []string{"run", "--rm"})+"\n", out)
	assert.Equal(t, "agentic: tool container:\n\n", logs.String())
}

func Test_shellStyle_command(t *testing.T) {
	t.Run("puts each arg after the subcommand on its own line", func(t *testing.T) {
		// Act
		cmd := posixShell.command("docker", []string{"run", "--rm", "--env=A=b c"})

		// Assert
		assert.Equal(t, "docker run \\\n  --rm \\\n  '--env=A=b c'", cmd)
	})

	t.Run("uses the style's continuation", func(t *testing.T) {
		// Act
		cmd := powerShell.command("docker", []string{"run", "--rm"})

		// Assert
		assert.Equal(t, "docker run `\n  --rm", cmd)
	})

	t.Run("keeps a lone subcommand on one line", func(t *testing.T) {
		// Act
		cmd := posixShell.command("docker", []string{"run"})

		// Assert
		assert.Equal(t, "docker run", cmd)
	})
}

func Test_posixQuote(t *testing.T) {
	t.Run("leaves commas as is", func(t *testing.T) {
		// Act
		got := posixQuote("--env=A=b,c")

		// Assert
		assert.Equal(t, "--env=A=b,c", got)
	})

	t.Run("escapes single quotes", func(t *testing.T) {
		// Act
		got := posixQuote("it's")

		// Assert
		assert.Equal(t, `'it'\''s'`, got)
	})
}

func Test_powerShellQuote(t *testing.T) {
	t.Run("leaves backslashes as is", func(t *testing.T) {
		// Act
		got := powerShellQuote(`--volume=C:\Users\dev:/workspace`)

		// Assert
		assert.Equal(t, `--volume=C:\Users\dev:/workspace`, got)
	})

	t.Run("quotes commas", func(t *testing.T) {
		// Act
		got := powerShellQuote("--env=A=b,c")

		// Assert
		assert.Equal(t, "'--env=A=b,c'", got)
	})

	t.Run("doubles single quotes", func(t *testing.T) {
		// Act
		got := powerShellQuote("it's")

		// Assert
		assert.Equal(t, "'it''s'", got)
	})
}

func Test_singleQuote(t *testing.T) {
	t.Run("leaves letters, digits and safe chars as is", func(t *testing.T) {
		// Act
		got := singleQuote("--env=A_1=b.c/d:e", "", "''")

		// Assert
		assert.Equal(t, "--env=A_1=b.c/d:e", got)
	})

	t.Run("leaves extra safe chars as is", func(t *testing.T) {
		// Act
		got := singleQuote("a,b", ",", "''")

		// Assert
		assert.Equal(t, "a,b", got)
	})

	t.Run("quotes other chars", func(t *testing.T) {
		// Act
		got := singleQuote("--env=B=$X y", "", "''")

		// Assert
		assert.Equal(t, "'--env=B=$X y'", got)
	})

	t.Run("quotes an empty arg", func(t *testing.T) {
		// Act
		got := singleQuote("", "", "''")

		// Assert
		assert.Equal(t, "''", got)
	})

	t.Run("escapes single quotes", func(t *testing.T) {
		// Act
		got := singleQuote("it's", "", "Q")

		// Assert
		assert.Equal(t, "'itQs'", got)
	})
}
