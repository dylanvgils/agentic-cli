package credentials

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSource(t *testing.T) {
	home := stubHome(t)

	t.Run("absolute path is a file source", func(t *testing.T) {
		// Act
		source, err := ParseSource("/secrets/../example/token")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, filepath.Clean("/example/token"), source.String())
	})

	t.Run("home prefixes are expanded", func(t *testing.T) {
		for _, spec := range []string{"~/.example/token", "$HOME/.example/token", "${HOME}/.example/token"} {
			// Act
			source, err := ParseSource(spec)

			// Assert
			require.NoError(t, err, spec)
			assert.Equal(t, filepath.Join(home, ".example", "token"), source.String(), spec)
		}
	})

	t.Run("tilde inside a name is not expanded", func(t *testing.T) {
		// Act
		_, err := ParseSource("~other/token")

		// Assert
		assert.ErrorContains(t, err, "must be absolute")
	})

	t.Run("relative path is rejected", func(t *testing.T) {
		// Act
		_, err := ParseSource("example/token")

		// Assert
		assert.ErrorContains(t, err, "must be absolute")
	})

	t.Run("scheme is unsupported", func(t *testing.T) {
		// Act
		_, err := ParseSource("keychain:example-item")

		// Assert
		assert.ErrorContains(t, err, `unsupported secret source "keychain:"`)
	})
}

func Test_fileSource_Resolve(t *testing.T) {
	t.Run("reads the file", func(t *testing.T) {
		// Arrange
		source := fileSource{path: writeSecret(t, "test-secret\n")}

		// Act
		data, err := source.Resolve()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "test-secret\n", string(data))
	})

	t.Run("missing file is an error", func(t *testing.T) {
		// Arrange
		source := fileSource{path: filepath.Join(t.TempDir(), "missing")}

		// Act
		_, err := source.Resolve()

		// Assert
		assert.ErrorContains(t, err, "read secret")
	})

	t.Run("directory is rejected", func(t *testing.T) {
		// Arrange
		source := fileSource{path: t.TempDir()}

		// Act
		_, err := source.Resolve()

		// Assert
		assert.ErrorContains(t, err, "not a regular file")
	})

	t.Run("oversized file is rejected", func(t *testing.T) {
		// Arrange
		source := fileSource{path: writeSecret(t, strings.Repeat("a", maxSecretSize+1))}

		// Act
		_, err := source.Resolve()

		// Assert
		assert.ErrorContains(t, err, "larger than")
	})
}
