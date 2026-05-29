package docker

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCmd(t *testing.T) {
	t.Run("captures output", func(t *testing.T) {
		// Arrange
		stubDocker(t, `printf '%s\n' "$@"`)

		// Act
		out, err := RunCmd("images", "--quiet")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "images\n--quiet\n", out)
	})

	t.Run("returns error on failure", func(t *testing.T) {
		// Arrange
		stubDocker(t, `exit 1`)

		// Act
		_, err := RunCmd("bad-command")

		// Assert
		assert.Error(t, err)
	})
}

func TestRun(t *testing.T) {
	t.Run("pipes stdin", func(t *testing.T) {
		// Arrange
		stubDocker(t, `cat`)

		// Act
		out, err := Run(strings.NewReader("hello\n"), "build", "-")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "hello\n", out)
	})

	t.Run("nil stdin does not block", func(t *testing.T) {
		// Arrange
		stubDocker(t, `echo ok`)

		// Act
		out, err := Run(nil, "info")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "ok\n", out)
	})
}
