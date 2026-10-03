//go:build integration

package integration

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestToolContainer checks the kernel enforces the security and resource args, not just that they're passed.
func TestToolContainer(t *testing.T) {
	t.Run("root filesystem is read-only", func(t *testing.T) {
		// Act
		out := runInTool(t, "touch /itest 2>/dev/null && echo rootfs=rw || echo rootfs=ro")

		// Assert
		assert.Contains(t, out, "rootfs=ro")
	})

	t.Run("all capabilities dropped", func(t *testing.T) {
		// Act (bounding set, since CapEff of a non-root user is empty even without --cap-drop)
		out := runInTool(t, `echo "cap=$(awk '/^CapBnd/ {print $2}' /proc/self/status)"`)

		// Assert
		assert.Contains(t, out, "cap=0000000000000000")
	})

	t.Run("no new privileges", func(t *testing.T) {
		// Act
		out := runInTool(t, `echo "nnp=$(awk '/^NoNewPrivs/ {print $2}' /proc/self/status)"`)

		// Assert
		assert.Contains(t, out, "nnp=1")
	})

	t.Run("runs as host user", func(t *testing.T) {
		// Act
		out := runInTool(t, `echo "user=$(id -u):$(id -g)"`)

		// Assert
		assert.Contains(t, out, "user="+strconv.Itoa(os.Getuid())+":"+strconv.Itoa(os.Getgid()))
	})

	t.Run("pids limit applied", func(t *testing.T) {
		// Arrange
		skipWithoutCgroups(t)

		// Act
		out := runInTool(t, `echo "pids=$(cat /sys/fs/cgroup/pids.max)"`)

		// Assert
		assert.Contains(t, out, "pids="+testPidsLimit)
	})

	t.Run("memory limit applied", func(t *testing.T) {
		// Arrange
		skipWithoutCgroups(t)

		// Act
		out := runInTool(t, `echo "mem=$(cat /sys/fs/cgroup/memory.max)"`)

		// Assert
		assert.Contains(t, out, "mem=314572800") // testMemory in bytes
	})
}
