//go:build integration

package container

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToolMounts checks the tool container only gets the mounts it should, writable only where it should be.
func TestToolMounts(t *testing.T) {
	t.Run("no docker socket", func(t *testing.T) {
		// Act
		out := platform.NewToolRun(t).Run("for s in /var/run/docker.sock /run/docker.sock; do test -e $s && echo socket=$s; done; echo checked")

		// Assert
		assert.Contains(t, out, "checked")
		assert.NotContains(t, out, "socket=")
	})

	t.Run("read-only sub-path mount blocks writes inside writable workspace", func(t *testing.T) {
		// Arrange
		lockedDir := platform.TempDirIn(t, platform.WorkDir())
		locked := "/workspace/" + filepath.Base(lockedDir)
		script := writableScript("workspace", locked+"-ctl") + "; " + writableScript("locked", locked+"/x")

		// Act
		out := platform.NewToolRun(t).
			WithVolume(lockedDir + ":" + locked).
			Run(script)

		// Assert
		assert.Contains(t, out, "workspace=rw")
		assert.Contains(t, out, "locked=ro")
	})

	t.Run("extra bind mount is read-only unless rw", func(t *testing.T) {
		// Arrange
		defaultDir := platform.TempDirIn(t, platform.RootDir())
		rwDir := platform.TempDirIn(t, platform.RootDir())
		script := writableScript("default", "/mnt/default/x") + "; " + writableScript("opted", "/mnt/rw/x")

		// Act
		out := platform.NewToolRun(t).
			WithVolume(defaultDir + ":/mnt/default").
			WithVolume(rwDir + ":/mnt/rw:rw").
			Run(script)

		// Assert
		assert.Contains(t, out, "default=ro")
		assert.Contains(t, out, "opted=rw")
	})

	t.Run("mount through a workspace symlink is refused", func(t *testing.T) {
		// Arrange
		link := filepath.Join(platform.WorkDir(), ".git")
		require.NoError(t, os.Symlink(platform.TempDirIn(t, platform.RootDir()), link))
		t.Cleanup(func() { _ = os.Remove(link) })

		// Act
		out := platform.NewToolRun(t).
			WithVolume("./.git:/mnt/git").
			RunFailing("echo started")

		// Assert
		assert.Contains(t, out, "goes through workspace symlink")
		assert.NotContains(t, out, "started")
	})
}
