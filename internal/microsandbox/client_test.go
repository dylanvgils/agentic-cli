package microsandbox

import (
	"os"
	"path/filepath"
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

func TestClient_Available(t *testing.T) {
	t.Run("nil when msb is on PATH", func(t *testing.T) {
		// Arrange
		stubMsb(t, "true")

		// Act
		err := New().Available()

		// Assert
		assert.NoError(t, err)
	})

	t.Run("install hint when msb is missing", func(t *testing.T) {
		// Arrange
		t.Setenv("PATH", t.TempDir())

		// Act
		err := New().Available()

		// Assert
		assert.ErrorIs(t, err, errNotInstalled)
	})
}

func Test_execRunner_Run(t *testing.T) {
	t.Run("captures output", func(t *testing.T) {
		// Arrange
		stubMsb(t, `printf '%s\n' "$@"`)

		// Act
		out, err := execRunner{}.Run("images", "--quiet")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "images\n--quiet\n", out)
	})

	t.Run("returns output in the error on failure", func(t *testing.T) {
		// Arrange
		stubMsb(t, `echo boom; exit 1`)

		// Act
		_, err := execRunner{}.Run("load")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boom")
	})

	t.Run("refuses when msb is not installed", func(t *testing.T) {
		// Arrange
		t.Setenv("PATH", t.TempDir())

		// Act
		_, err := execRunner{}.Run("load")

		// Assert
		assert.ErrorIs(t, err, errNotInstalled)
	})
}

func Test_execRunner_RunWithStdin(t *testing.T) {
	t.Run("pipes stdin and captures output", func(t *testing.T) {
		// Arrange
		stubMsb(t, `printf '%s\n' "$@"; cat`)

		// Act
		out, err := execRunner{}.RunWithStdin(strings.NewReader("tar"), "load")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "load\ntar", out)
	})

	t.Run("refuses when msb is not installed", func(t *testing.T) {
		// Arrange
		t.Setenv("PATH", t.TempDir())

		// Act
		_, err := execRunner{}.RunWithStdin(strings.NewReader("tar"), "load")

		// Assert
		assert.ErrorIs(t, err, errNotInstalled)
	})
}

func Test_execRunner_RunInteractive(t *testing.T) {
	t.Run("adds env to msb's environment", func(t *testing.T) {
		// Arrange
		out := filepath.Join(t.TempDir(), "env")
		stubMsb(t, `echo "$EXAMPLE_TOKEN" > "`+out+`"`)

		// Act
		err := execRunner{}.RunInteractive([]string{"EXAMPLE_TOKEN=test-secret"}, "run", "img")

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Equal(t, "test-secret\n", string(got))
	})

	t.Run("missing msb", func(t *testing.T) {
		// Arrange
		t.Setenv("PATH", t.TempDir())

		// Act
		err := execRunner{}.RunInteractive(nil, "run", "img")

		// Assert
		assert.ErrorIs(t, err, errNotInstalled)
	})
}
