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

// TestMicrosandboxAgenticProxy checks a VM run with agentic-proxy reaches out only through the run's published proxy.
func TestMicrosandboxAgenticProxy(t *testing.T) {
	t.Run("allowlist blocks other hosts and logs them", func(t *testing.T) {
		// Arrange
		script := reachableScript("allowed", "example.com") + "; " + reachableScript("other", "example.org")
		run := platform.NewToolRun(t).
			InMicrosandbox().
			WithProxyBackend("agentic").
			Allowing("example.com").
			WithProxy()

		// Act
		out := run.Run(script)

		// Assert
		assert.Contains(t, out, "allowed=ok")
		assert.Contains(t, out, "other=blocked")
		log, err := os.ReadFile(run.ProxyLogFile())
		require.NoError(t, err)
		assert.Contains(t, string(log), `"host":"example.org"`)
	})

	t.Run("monitor mode logs without blocking", func(t *testing.T) {
		// Arrange
		run := platform.NewToolRun(t).
			InMicrosandbox().
			WithProxyBackend("agentic").
			Allowing("example.com").
			WithProxyMonitor()

		// Act
		out := run.Run(reachableScript("other", "example.org"))

		// Assert
		assert.Contains(t, out, "other=ok")
		log, err := os.ReadFile(run.ProxyLogFile())
		require.NoError(t, err)
		assert.Contains(t, string(log), `"host":"example.org"`)
	})

	t.Run("egress around the proxy fails", func(t *testing.T) {
		// Arrange
		script := "env -u HTTPS_PROXY -u https_proxy -u HTTP_PROXY -u http_proxy sh -c '" + reachableScript("direct", "example.com") + "'"

		// Act
		out := platform.NewToolRun(t).
			InMicrosandbox().
			WithProxyBackend("agentic").
			Allowing("example.com").
			WithProxy().
			Run(script)

		// Assert
		assert.Contains(t, out, "direct=blocked")
	})

	t.Run("the proxy refuses a request without the run's token", func(t *testing.T) {
		// Arrange
		script := `url=$(echo "$HTTPS_PROXY" | sed 's#//[^@]*@#//#'); ` +
			`echo "connect=$(curl --silent --max-time 20 --output /dev/null --write-out '%{http_connect}' --proxy "$url" https://example.com)"`

		// Act
		out := platform.NewToolRun(t).
			InMicrosandbox().
			WithProxyBackend("agentic").
			Allowing("example.com").
			WithProxy().
			Run(script)

		// Assert
		assert.Contains(t, out, "connect=407")
	})
}
