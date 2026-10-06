//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToolMounts checks the tool container only gets the mounts it should, writable only where it should be.
func TestToolMounts(t *testing.T) {
	t.Run("no docker socket", func(t *testing.T) {
		// Act
		out := runInTool(t, "for s in /var/run/docker.sock /run/docker.sock; do test -e $s && echo socket=$s; done; echo checked")

		// Assert
		assert.Contains(t, out, "checked")
		assert.NotContains(t, out, "socket=")
	})

	t.Run("secret is read-only", func(t *testing.T) {
		// Arrange
		secret := filepath.Join(tempDirIn(t, rootDir), "token")
		require.NoError(t, os.WriteFile(secret, []byte("s3cret"), 0o600))

		// Act
		out := runInTool(t, `cat /run/secrets/itest; echo; echo x > /run/secrets/itest 2>/dev/null && echo secret=rw || echo secret=ro`, "--secret", "itest:"+secret)

		// Assert
		assert.Contains(t, out, "s3cret")
		assert.Contains(t, out, "secret=ro")
	})

	t.Run("read-only mount blocks writes inside writable workspace", func(t *testing.T) {
		// Arrange
		locked := filepath.Base(tempDirIn(t, workDir))
		script := `touch /workspace/` + locked + `-ctl && rm /workspace/` + locked + `-ctl && echo workspace=rw; ` +
			`touch /workspace/` + locked + `/x 2>/dev/null && echo locked=rw || echo locked=ro`

		// Act
		out := runInTool(t, script, "--read-only-mount", locked)

		// Assert
		assert.Contains(t, out, "workspace=rw")
		assert.Contains(t, out, "locked=ro")
	})

	t.Run("extra bind mount is read-only unless rw", func(t *testing.T) {
		// Arrange
		defaultDir := tempDirIn(t, rootDir)
		rwDir := tempDirIn(t, rootDir)
		script := `touch /mnt/default/x 2>/dev/null && echo default=rw || echo default=ro; ` +
			`touch /mnt/rw/x 2>/dev/null && echo opted=rw || echo opted=ro`

		// Act
		out := runInTool(t, script, "-v", defaultDir+":/mnt/default", "-v", rwDir+":/mnt/rw:rw")

		// Assert
		assert.Contains(t, out, "default=ro")
		assert.Contains(t, out, "opted=rw")
	})
}
