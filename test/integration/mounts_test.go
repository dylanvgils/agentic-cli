//go:build integration

package integration

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestToolMounts checks the tool container only gets the mounts it should, writable only where it should be.
func TestToolMounts(t *testing.T) {
	t.Run("no docker socket", func(t *testing.T) {
		// Act
		out := newToolRun(t).run("for s in /var/run/docker.sock /run/docker.sock; do test -e $s && echo socket=$s; done; echo checked")

		// Assert
		assert.Contains(t, out, "checked")
		assert.NotContains(t, out, "socket=")
	})

	t.Run("read-only sub-path mount blocks writes inside writable workspace", func(t *testing.T) {
		// Arrange
		lockedDir := tempDirIn(t, workDir)
		locked := "/workspace/" + filepath.Base(lockedDir)
		script := writableScript("workspace", locked+"-ctl") + "; " + writableScript("locked", locked+"/x")

		// Act
		out := newToolRun(t).
			withVolume(lockedDir + ":" + locked).
			run(script)

		// Assert
		assert.Contains(t, out, "workspace=rw")
		assert.Contains(t, out, "locked=ro")
	})

	t.Run("extra bind mount is read-only unless rw", func(t *testing.T) {
		// Arrange
		defaultDir := tempDirIn(t, rootDir)
		rwDir := tempDirIn(t, rootDir)
		script := writableScript("default", "/mnt/default/x") + "; " + writableScript("opted", "/mnt/rw/x")

		// Act
		out := newToolRun(t).
			withVolume(defaultDir + ":/mnt/default").
			withVolume(rwDir + ":/mnt/rw:rw").
			run(script)

		// Assert
		assert.Contains(t, out, "default=ro")
		assert.Contains(t, out, "opted=rw")
	})
}
