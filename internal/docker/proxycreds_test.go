package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_tarFiles(t *testing.T) {
	// Arrange
	files := []archiveFile{{"ca.pem", []byte("test-cert")}, {"credentials.json", []byte("[]")}}

	// Act
	archive, err := tarFiles(files, 1234, 5678)

	// Assert
	require.NoError(t, err)
	got := readTar(t, archive)
	require.Len(t, got, 2)
	for _, file := range files {
		entry := got[file.name]
		assert.Equal(t, file.content, entry.content)
		assert.Equal(t, int64(0o600), entry.header.Mode)
		assert.Equal(t, 1234, entry.header.Uid)
		assert.Equal(t, 5678, entry.header.Gid)
	}
}

func Test_parseUserGroup(t *testing.T) {
	t.Run("splits uid and gid", func(t *testing.T) {
		// Act
		uid, gid, err := parseUserGroup("1234:5678")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 1234, uid)
		assert.Equal(t, 5678, gid)
	})

	t.Run("malformed values are errors", func(t *testing.T) {
		for _, value := range []string{"1234", "abc:5678", "1234:abc"} {
			// Act
			_, _, err := parseUserGroup(value)

			// Assert
			assert.Error(t, err, value)
		}
	})
}
