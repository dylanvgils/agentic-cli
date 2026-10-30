//go:build integration

package microsandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMicrosandboxRun checks a tool runs in its own VM with the run's mounts, limits and allowlist.
func TestMicrosandboxRun(t *testing.T) {
	t.Run("runs in its own kernel", func(t *testing.T) {
		// Arrange
		host, err := exec.Command("uname", "-r").Output()
		require.NoError(t, err)

		// Act
		out := platform.NewToolRun(t).
			InMicrosandbox().
			Run("echo kernel=$(uname -r)")

		// Assert
		assert.Contains(t, out, "kernel=")
		assert.NotContains(t, out, "kernel="+strings.TrimSpace(string(host)))
	})

	t.Run("workspace is writable", func(t *testing.T) {
		// Arrange
		name := filepath.Base(platform.TempDirIn(t, platform.WorkDir()))

		// Act
		platform.NewToolRun(t).
			InMicrosandbox().
			Run("echo written > /workspace/" + name + "/out")

		// Assert
		got, err := os.ReadFile(filepath.Join(platform.WorkDir(), name, "out"))
		require.NoError(t, err)
		assert.Equal(t, "written\n", string(got))
	})

	t.Run("extra bind mount is read-only unless rw", func(t *testing.T) {
		// Arrange
		defaultDir := platform.TempDirIn(t, platform.RootDir())
		rwDir := platform.TempDirIn(t, platform.RootDir())
		script := writableScript("default", "/mnt/default/x") + "; " + writableScript("opted", "/mnt/rw/x")

		// Act
		out := platform.NewToolRun(t).
			InMicrosandbox().
			WithVolume(defaultDir + ":/mnt/default").
			WithVolume(rwDir + ":/mnt/rw:rw").
			Run(script)

		// Assert
		assert.Contains(t, out, "default=ro")
		assert.Contains(t, out, "opted=rw")
	})

	t.Run("limits apply to the vm", func(t *testing.T) {
		// Act
		out := platform.NewToolRun(t).
			InMicrosandbox().
			Run("echo cpus=$(nproc); awk '/MemTotal/ { print \"memkb=\" $2 }' /proc/meminfo")

		// Assert
		// msb takes whole CPUs, so the work dir's 1.5 rounds up
		assert.Contains(t, out, "cpus=2")
		memKB := valueAfter(t, out, "memkb=")
		limitMB, err := strconv.Atoi(strings.TrimSuffix(platform.Memory, "m"))
		require.NoError(t, err)
		// The guest sees a few percent more than the limit
		limitKB := float64(limitMB * 1024)
		assert.InDelta(t, limitKB, float64(memKB), limitKB*0.1)
	})

	t.Run("allowlist blocks other hosts", func(t *testing.T) {
		// Arrange
		script := reachableScript("allowed", "example.com") + "; " + reachableScript("other", "example.org")

		// Act
		out := platform.NewToolRun(t).
			InMicrosandbox().
			Allowing("example.com").
			WithProxy().
			Run(script)

		// Assert
		assert.Contains(t, out, "allowed=ok")
		assert.Contains(t, out, "other=blocked")
	})

	t.Run("second run reuses the loaded image", func(t *testing.T) {
		// Arrange
		platform.NewToolRun(t).
			InMicrosandbox().
			Run("true")

		// Act
		out := platform.NewToolRun(t).
			InMicrosandbox().
			Run("true")

		// Assert
		assert.NotContains(t, out, "into microsandbox")
	})
}
