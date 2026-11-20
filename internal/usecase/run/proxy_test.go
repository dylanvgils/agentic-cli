package run

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_proxyRules(t *testing.T) {
	rc := &config.AgenticRC{}
	rc.Run.Proxy.AllowedPaths = []config.RCAllowedPath{{Host: "raw.example.test", Paths: []string{"/a"}}}
	c := runConfig{
		Tool: tools.ToolConfig{Runtime: tools.RuntimeConfig{AllowedHosts: []string{"api.example.test"}}},
		RC:   rc,
	}

	t.Run("a runtime with path rules keeps the paths", func(t *testing.T) {
		// Arrange
		withPaths := c
		withPaths.Runtime.Caps.PathRules = true

		// Act
		rules := proxyRules(withPaths)

		// Assert
		assert.Equal(t, []sandbox.AllowRule{{Host: "api.example.test"}, {Host: "raw.example.test", Paths: []string{"/a"}}}, rules)
	})

	t.Run("a runtime without path rules allows their hosts whole", func(t *testing.T) {
		// Act
		rules := proxyRules(c)

		// Assert
		assert.Equal(t, []sandbox.AllowRule{{Host: "api.example.test"}, {Host: "raw.example.test"}}, rules)
	})
}

func Test_proxyIntegrations(t *testing.T) {
	rc := &config.AgenticRC{}
	flags := resolve.ProxyInput{Integrations: []string{"maven"}}
	caps := sandbox.Features{ProxyIntegrations: true}

	t.Run("returns them with the proxy on a runtime that supports them", func(t *testing.T) {
		// Act
		integrations := proxyIntegrations(flags, rc, sandbox.EgressEnforce, caps)

		// Assert
		assert.Equal(t, []string{"maven"}, integrations)
	})

	t.Run("none without the proxy", func(t *testing.T) {
		// Act
		integrations := proxyIntegrations(flags, rc, sandbox.EgressOff, caps)

		// Assert
		assert.Empty(t, integrations)
	})

	t.Run("none on a runtime without them", func(t *testing.T) {
		// Act
		integrations := proxyIntegrations(flags, rc, sandbox.EgressEnforce, sandbox.Features{})

		// Assert
		assert.Empty(t, integrations)
	})
}

func Test_proxyLogFile(t *testing.T) {
	t.Run("returns empty string when proxy is disabled", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{
			RunID:    "0123456789ab",
			ToolHome: "/example.test/home",
		}

		// Act
		file := proxyLogFile(rs, time.Now())

		// Assert
		assert.Empty(t, file)
	})

	t.Run("names the proxy log in the run's log dir without creating it", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		rs := sandbox.RunSpec{
			RunID:    "0123456789ab",
			ToolHome: home,
			Egress:   sandbox.EgressPolicy{Mode: sandbox.EgressEnforce},
		}
		start := time.Date(2026, 11, 20, 15, 30, 45, 0, time.Local)

		// Act
		file := proxyLogFile(rs, start)

		// Assert
		assert.Equal(t, filepath.Join(home, logging.DirName, "20261120-153045-0123456789ab", proxy.LogFileName), file)
		assert.NoDirExists(t, filepath.Dir(file))
	})
}

func Test_prepareProxyLogDir(t *testing.T) {
	t.Run("creates the run log dir", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		runDir := logging.Dir(home, "0123456789ab", time.Now())

		// Act
		err := prepareProxyLogDir(runDir, home)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, runDir)
	})

	t.Run("prunes run log dirs older than the retention window", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		require.NoError(t, os.WriteFile(config.ConfigFile(home), []byte(`{"log_retention_days":1}`), 0o640))
		oldDir := logging.Dir(home, "ba9876543210", time.Now().Add(-48*time.Hour))
		require.NoError(t, os.MkdirAll(oldDir, 0o750))
		runDir := logging.Dir(home, "0123456789ab", time.Now())

		// Act
		err := prepareProxyLogDir(runDir, home)

		// Assert
		require.NoError(t, err)
		assert.NoDirExists(t, oldDir)
		assert.DirExists(t, runDir)
	})
}
