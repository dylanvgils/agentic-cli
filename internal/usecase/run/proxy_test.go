package run

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_proxyRules(t *testing.T) {
	rc := &config.AgenticRC{}
	rc.Run.Proxy.AllowedPaths = []config.RCAllowedPath{{Host: "raw.example.test", Paths: []string{"/a"}}}
	toolConfig := tools.ToolConfig{Runtime: tools.RuntimeConfig{AllowedHosts: []string{"api.example.test"}}}

	t.Run("a runtime with path rules keeps the paths", func(t *testing.T) {
		// Act
		rules := proxyRules(toolConfig, Input{}, rc, sandbox.Capabilities{PathRules: true})

		// Assert
		assert.Equal(t, []sandbox.AllowRule{{Host: "api.example.test"}, {Host: "raw.example.test", Paths: []string{"/a"}}}, rules)
	})

	t.Run("a runtime without path rules allows their hosts whole", func(t *testing.T) {
		// Act
		rules := proxyRules(toolConfig, Input{}, rc, sandbox.Capabilities{})

		// Assert
		assert.Equal(t, []sandbox.AllowRule{{Host: "api.example.test"}, {Host: "raw.example.test"}}, rules)
	})
}

func Test_proxyIntegrations(t *testing.T) {
	rc := &config.AgenticRC{}
	flags := resolve.ProxyInput{Integrations: []string{"maven"}}
	caps := sandbox.Capabilities{ProxyIntegrations: true}

	t.Run("returns them with the proxy on a runtime that supports them", func(t *testing.T) {
		// Act
		integrations := proxyIntegrations(flags, rc, sandbox.ProxyEnforce, caps)

		// Assert
		assert.Equal(t, []string{"maven"}, integrations)
	})

	t.Run("none without the proxy", func(t *testing.T) {
		// Act
		integrations := proxyIntegrations(flags, rc, sandbox.ProxyOff, caps)

		// Assert
		assert.Empty(t, integrations)
	})

	t.Run("none on a runtime without them", func(t *testing.T) {
		// Act
		integrations := proxyIntegrations(flags, rc, sandbox.ProxyEnforce, sandbox.Capabilities{})

		// Assert
		assert.Empty(t, integrations)
	})
}

func Test_proxyRetentionDays(t *testing.T) {
	t.Run("uses the value configured in agentic.json", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		require.NoError(t, os.WriteFile(config.ConfigFile(home), []byte(`{"proxy_log_retention_days":30}`), 0o640))

		// Act
		result := proxyRetentionDays(home)

		// Assert
		assert.Equal(t, 30, result)
	})

	t.Run("falls back to default when not configured", func(t *testing.T) {
		// Arrange
		home := t.TempDir()

		// Act
		result := proxyRetentionDays(home)

		// Assert
		assert.Equal(t, defaultProxyLogRetentionDays, result)
	})
}

func Test_proxyLogFile(t *testing.T) {
	t.Run("returns empty string when proxy is disabled", func(t *testing.T) {
		// Act
		file, err := proxyLogFile("/example.test/home", false)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, file)
	})

	t.Run("returns a new proxy log in the tool home's logs dir without creating it", func(t *testing.T) {
		// Arrange
		home := t.TempDir()

		// Act
		file, err := proxyLogFile(home, true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, config.LogsDirName), filepath.Dir(file))
		assert.True(t, strings.HasPrefix(filepath.Base(file), proxy.LogFilePrefix))
		assert.Equal(t, ".jsonl", filepath.Ext(file))
		assert.NoDirExists(t, filepath.Dir(file))
	})

	t.Run("names each run's log differently", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		first, err := proxyLogFile(home, true)
		require.NoError(t, err)

		// Act
		second, err := proxyLogFile(home, true)

		// Assert
		require.NoError(t, err)
		assert.NotEqual(t, first, second)
	})
}

func Test_prepareProxyLogDir(t *testing.T) {
	t.Run("creates the log dir", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		logDir := filepath.Join(home, config.LogsDirName)

		// Act
		err := prepareProxyLogDir(logDir, home)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, logDir)
	})

	t.Run("prunes logs older than the retention window", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		require.NoError(t, os.WriteFile(config.ConfigFile(home), []byte(`{"proxy_log_retention_days":1}`), 0o640))
		logDir := filepath.Join(home, config.LogsDirName)
		require.NoError(t, os.MkdirAll(logDir, 0o750))
		oldLog := filepath.Join(logDir, proxy.LogFilePrefix+"old.jsonl")
		require.NoError(t, os.WriteFile(oldLog, []byte("{}\n"), 0o644))
		old := time.Now().Add(-48 * time.Hour)
		require.NoError(t, os.Chtimes(oldLog, old, old))

		// Act
		err := prepareProxyLogDir(logDir, home)

		// Assert
		require.NoError(t, err)
		assert.NoFileExists(t, oldLog)
	})
}
