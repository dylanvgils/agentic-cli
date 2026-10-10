package msb

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	// Act
	client := New()

	// Assert
	assert.Equal(t, execRunner{}, client.runner)
}

func Test_execRunner_Run(t *testing.T) {
	t.Run("pipes stdin and captures output", func(t *testing.T) {
		// Arrange
		stubMsb(t, `printf '%s\n' "$@"; cat`)

		// Act
		out, err := execRunner{}.Run(strings.NewReader("tar"), "load")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "load\ntar", out)
	})

	t.Run("returns output in the error on failure", func(t *testing.T) {
		// Arrange
		stubMsb(t, `echo boom; exit 1`)

		// Act
		_, err := execRunner{}.Run(nil, "load")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boom")
	})

	t.Run("refuses when msb is not installed", func(t *testing.T) {
		// Arrange
		t.Setenv("PATH", t.TempDir())

		// Act
		_, err := execRunner{}.Run(nil, "load")

		// Assert
		assert.ErrorIs(t, err, errNotInstalled)
	})
}

func Test_execRunner_RunInteractive(t *testing.T) {
	// Arrange
	t.Setenv("PATH", t.TempDir())

	// Act
	err := execRunner{}.RunInteractive("run", "img")

	// Assert
	assert.ErrorIs(t, err, errNotInstalled)
}
