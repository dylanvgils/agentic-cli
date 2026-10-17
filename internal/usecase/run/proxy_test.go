package run

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func Test_printProxySummary(t *testing.T) {
	t.Run("enforced mode reports blocked hosts", func(t *testing.T) {
		// Arrange
		spec := sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, LogFile: writeProxyLog(t,
			`{"host":"api.anthropic.com","decision":"allow"}`,
			`{"host":"evil.com","decision":"deny"}`,
		)}
		var buf strings.Builder

		// Act
		printProxySummary(logging.New(&buf), spec)

		// Assert
		assert.Contains(t, buf.String(), "agentic: proxy blocked 1 request(s) to: evil.com")
		assert.NotContains(t, buf.String(), "monitor mode")
	})

	t.Run("monitor mode reports would-be-blocked hosts", func(t *testing.T) {
		// Arrange
		spec := sandbox.ProxySpec{Mode: sandbox.ProxyMonitor, LogFile: writeProxyLog(t,
			`{"host":"api.anthropic.com","decision":"allow"}`,
			`{"host":"evil.com","decision":"deny"}`,
		)}
		var buf strings.Builder

		// Act
		printProxySummary(logging.New(&buf), spec)

		// Assert
		assert.Contains(t, buf.String(), "agentic: proxy (monitor mode) observed 2 request(s); 1 would be blocked under the current allowlist: evil.com")
		assert.Contains(t, buf.String(), "--proxy-monitor")
	})

	t.Run("warns about credential hosts reached without the placeholder", func(t *testing.T) {
		// Arrange
		spec := sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, LogFile: writeProxyLog(t,
			`{"host":"api.example.test","decision":"allow","intercepted":true}`,
			`{"host":"api.example.test","decision":"allow","intercepted":true,"reason":"no-placeholder"}`,
		)}
		var buf strings.Builder

		// Act
		printProxySummary(logging.New(&buf), spec)

		// Assert
		assert.Contains(t, buf.String(), "agentic: warning: proxy saw 1 connection(s) to credential hosts without the placeholder, so they likely ran unauthenticated: api.example.test")
		assert.NotContains(t, buf.String(), "blocked")
	})

	t.Run("guard refusals are reported apart from allowlist blocks", func(t *testing.T) {
		// Arrange
		spec := sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, LogFile: writeProxyLog(t,
			`{"host":"evil.com","decision":"deny"}`,
			`{"host":"localhost","decision":"deny","reason":"blocked-address"}`,
		)}
		var buf strings.Builder

		// Act
		printProxySummary(logging.New(&buf), spec)

		// Assert
		assert.Contains(t, buf.String(), "agentic: proxy blocked 1 request(s) to: evil.com")
		assert.Contains(t, buf.String(), "agentic: proxy refused 1 request(s) to local or private addresses: localhost")
	})

	t.Run("monitor mode reports guard refusals as refused, not would-be-blocked", func(t *testing.T) {
		// Arrange
		spec := sandbox.ProxySpec{Mode: sandbox.ProxyMonitor, LogFile: writeProxyLog(t,
			`{"host":"localhost","decision":"deny","enforced":true,"reason":"blocked-address"}`,
		)}
		var buf strings.Builder

		// Act
		printProxySummary(logging.New(&buf), spec)

		// Assert
		assert.Contains(t, buf.String(), "agentic: proxy refused 1 request(s) to local or private addresses: localhost")
		assert.NotContains(t, buf.String(), "would be blocked")
	})

	t.Run("nothing denied prints nothing", func(t *testing.T) {
		// Arrange
		spec := sandbox.ProxySpec{Mode: sandbox.ProxyMonitor, LogFile: writeProxyLog(t, `{"host":"api.anthropic.com","decision":"allow"}`)}
		var buf strings.Builder

		// Act
		printProxySummary(logging.New(&buf), spec)

		// Assert
		assert.Empty(t, buf.String())
	})
}

func Test_readProxyLog(t *testing.T) {
	t.Run("collects unique denied hosts in first-seen order and their total", func(t *testing.T) {
		// Arrange
		path := writeProxyLog(t,
			`{"host":"api.anthropic.com","decision":"allow"}`,
			`{"host":"evil.com","decision":"deny"}`,
			`{"host":"evil.com","decision":"deny"}`,
			`{"host":"tracker.net","decision":"deny"}`,
		)

		// Act
		summary := readProxyLog(path)

		// Assert
		assert.Equal(t, []string{"evil.com", "tracker.net"}, summary.denied.hosts)
		assert.Equal(t, 3, summary.denied.count)
		assert.Equal(t, 4, summary.attempts)
	})

	t.Run("sorts each entry into one kind", func(t *testing.T) {
		// Arrange
		path := writeProxyLog(t,
			`{"host":"down.example.test","decision":"deny","reason":"upstream-failed"}`,
			`{"host":"local.example.test","decision":"deny","reason":"blocked-address"}`,
			`{"host":"api.example.test","decision":"deny","intercepted":true,"reason":"no-placeholder"}`,
		)

		// Act
		summary := readProxyLog(path)

		// Assert
		assert.Equal(t, []string{"down.example.test"}, summary.denied.hosts, "a denied host whose upstream failed in monitor mode still counts")
		assert.Equal(t, []string{"local.example.test"}, summary.refused.hosts)
		assert.Equal(t, []string{"api.example.test"}, summary.missed.hosts)
	})

	t.Run("counts a tunnel with failed requests as one attempt", func(t *testing.T) {
		// Arrange
		path := writeProxyLog(t,
			`{"host":"api.example.test","decision":"allow","intercepted":true}`,
			`{"host":"api.example.test","decision":"allow","intercepted":true,"reason":"upstream-failed"}`,
			`{"host":"api.example.test","decision":"allow","intercepted":true,"reason":"no-placeholder"}`,
		)

		// Act
		summary := readProxyLog(path)

		// Assert
		assert.Equal(t, 1, summary.attempts)
	})

	t.Run("hosts are made safe to print", func(t *testing.T) {
		// Arrange
		path := writeProxyLog(t, `{"host":"evil\u202e.test","decision":"deny"}`)

		// Act
		summary := readProxyLog(path)

		// Assert
		assert.Equal(t, []string{`"evil\u202e.test"`}, summary.denied.hosts)
	})

	t.Run("skips lines it can't parse", func(t *testing.T) {
		// Arrange
		path := writeProxyLog(t, `not json`, `{"host":"evil.com","decision":"deny"}`)

		// Act
		summary := readProxyLog(path)

		// Assert
		assert.Equal(t, 1, summary.denied.count)
		assert.Equal(t, 1, summary.attempts)
	})

	t.Run("missing log yields nothing", func(t *testing.T) {
		// Act
		summary := readProxyLog(filepath.Join(t.TempDir(), "absent.jsonl"))

		// Assert
		assert.Equal(t, proxyLogSummary{}, summary)
	})
}

func Test_repeatsAttempt(t *testing.T) {
	cases := map[string]struct {
		entry proxy.Entry
		want  bool
	}{
		"intercepted tunnel":             {proxy.Entry{Intercepted: true}, false},
		"failed request inside a tunnel": {proxy.Entry{Intercepted: true, Reason: proxy.ReasonUpstreamFailed}, true},
		"missed placeholder":             {proxy.Entry{Intercepted: true, Reason: proxy.ReasonNoPlaceholder}, true},
		"failed plain tunnel":            {proxy.Entry{Reason: proxy.ReasonUpstreamFailed}, false},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			got := repeatsAttempt(c.entry)

			// Assert
			assert.Equal(t, c.want, got)
		})
	}
}
