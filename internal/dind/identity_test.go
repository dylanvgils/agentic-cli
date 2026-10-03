package dind

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIdentity(t *testing.T) {
	t.Run("uses the host uid and gid", func(t *testing.T) {
		// Act
		id, err := NewIdentity("1234:5678")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "1234:5678", id.UserGroup())
	})

	t.Run("refuses root", func(t *testing.T) {
		// Act
		_, err := NewIdentity("0:0")

		// Assert
		require.ErrorContains(t, err, "refuses to run as root")
	})
}

func TestIdentity_files(t *testing.T) {
	t.Run("maps the host uid to the dedicated subordinate range only", func(t *testing.T) {
		// Arrange
		id := Identity{uid: "1234", gid: "5678"}

		// Act
		files := id.files()

		// Assert
		assert.Equal(t, "rootless:2000000000:65536\n", files["subuid"])
		assert.Equal(t, "rootless:2000000000:65536\n", files["subgid"])
		assert.Contains(t, files["passwd"], "rootless:x:1234:5678::/home/rootless:/bin/sh\n")
		assert.Contains(t, files["group"], "rootless:x:5678:\n")
	})

	t.Run("primary group root is not duplicated", func(t *testing.T) {
		// Arrange
		id := Identity{uid: "1234", gid: "0"}

		// Act
		files := id.files()

		// Assert
		assert.Equal(t, "root:x:0:\n", files["group"])
	})
}

func TestWriteIdentityFiles(t *testing.T) {
	// Arrange
	dir := filepath.Join(t.TempDir(), "etc")

	// Act
	err := WriteIdentityFiles(dir, Identity{uid: "1234", gid: "5678"})

	// Assert
	require.NoError(t, err)
	for _, name := range IdentityFiles {
		_, statErr := os.Stat(filepath.Join(dir, name))
		assert.NoError(t, statErr, name)
	}
}
