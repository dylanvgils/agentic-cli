package docker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_currentDindIdentity(t *testing.T) {
	t.Run("uses the host uid and gid", func(t *testing.T) {
		// Arrange
		stubHostUserGroup(t, "1234:5678")

		// Act
		id, err := currentDindIdentity()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "1234:5678", id.userGroup())
	})

	t.Run("refuses root", func(t *testing.T) {
		// Arrange
		stubHostUserGroup(t, "0:0")

		// Act
		_, err := currentDindIdentity()

		// Assert
		require.ErrorContains(t, err, "refuses to run as root")
	})
}

func TestDindIdentityFiles(t *testing.T) {
	t.Run("maps the host uid to the dedicated subordinate range only", func(t *testing.T) {
		// Arrange
		id := dindIdentity{uid: "1234", gid: "5678"}

		// Act
		files := id.files()

		// Assert
		assert.Equal(t, "rootless:2000000000:65536\n", files["subuid"])
		assert.Equal(t, "rootless:2000000000:65536\n", files["subgid"])
		assert.Contains(t, files["passwd"], "rootless:x:1234:5678::/home/rootless:/bin/sh\n")
		assert.Contains(t, files["group"], "rootless:x:5678:\n")
		assert.NotContains(t, files["subuid"], "100000", "must not reuse the default useradd subuid range")
	})

	t.Run("primary group root is not duplicated", func(t *testing.T) {
		// Arrange
		id := dindIdentity{uid: "1234", gid: "0"}

		// Act
		files := id.files()

		// Assert
		assert.Equal(t, "root:x:0:\n", files["group"])
	})
}

func Test_writeDindIdentityFiles(t *testing.T) {
	// Arrange
	dir := filepath.Join(t.TempDir(), "etc")

	// Act
	err := writeDindIdentityFiles(dir, dindIdentity{uid: "1234", gid: "5678"})

	// Assert
	require.NoError(t, err)
	for _, name := range dindIdentityFiles {
		_, statErr := os.Stat(filepath.Join(dir, name))
		assert.NoError(t, statErr, name)
	}
}
