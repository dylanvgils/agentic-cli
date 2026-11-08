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

	for _, tc := range []struct {
		name    string
		spec    string
		wantErr string
	}{
		{name: "tilde inside a name is not expanded", spec: "~other/token", wantErr: "must be absolute"},
		{name: "relative path is rejected", spec: "example/token", wantErr: "must be absolute"},
		{name: "scheme is unsupported", spec: "keychain:example-item", wantErr: `unsupported secret source "keychain:"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			_, err := ParseSource(tc.spec)

			// Assert
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
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

	for _, tc := range []struct {
		name    string
		path    func(t *testing.T) string
		wantErr string
	}{
		{name: "missing file is an error", path: func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing") }, wantErr: "read secret"},
		{name: "directory is rejected", path: func(t *testing.T) string { return t.TempDir() }, wantErr: "not a regular file"},
		{name: "oversized file is rejected", path: func(t *testing.T) string { return writeSecret(t, strings.Repeat("a", maxSecretSize+1)) }, wantErr: "larger than"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			source := fileSource{path: tc.path(t)}

			// Act
			_, err := source.Resolve()

			// Assert
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}
