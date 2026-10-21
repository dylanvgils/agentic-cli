package run

import (
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/base/logging"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
)

func Test_printProxySummary(t *testing.T) {
	t.Run("sets what the log flagged apart from the tool output", func(t *testing.T) {
		// Arrange
		spec := sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, LogFile: writeProxyLog(t,
			`{"host":"evil.example.test","decision":"deny"}`,
			`{"host":"localhost","decision":"deny","reason":"blocked-address"}`,
			`{"host":"api.example.test","decision":"allow","intercepted":true,"reason":"no-placeholder"}`,
		)}
		var buf strings.Builder

		// Act
		printProxySummary(logging.New(&buf), spec)

		// Assert
		assert.True(t, strings.HasPrefix(buf.String(), "\nagentic: proxy blocked 1 request(s) to: evil.example.test"))
		assert.Contains(t, buf.String(), "proxy refused 1 request(s)")
		assert.Contains(t, buf.String(), "without the placeholder")
	})

	t.Run("nothing flagged prints nothing", func(t *testing.T) {
		// Arrange
		spec := sandbox.ProxySpec{Mode: sandbox.ProxyMonitor, LogFile: writeProxyLog(t, `{"host":"api.anthropic.com","decision":"allow"}`)}
		var buf strings.Builder

		// Act
		printProxySummary(logging.New(&buf), spec)

		// Assert
		assert.Empty(t, buf.String())
	})
}

func Test_printPaths(t *testing.T) {
	paths := proxy.HostCount{Hosts: []string{"raw.example.test/other"}, Count: 2}

	t.Run("enforced mode reports blocked paths", func(t *testing.T) {
		// Arrange
		var buf strings.Builder

		// Act
		printPaths(logging.New(&buf), paths, false)

		// Assert
		assert.Contains(t, buf.String(), "agentic: proxy blocked 2 request(s) to paths no rule allows: raw.example.test/other")
		assert.Contains(t, buf.String(), "[[run.proxy.allowed_paths]]")
	})

	t.Run("monitor mode reports would-be-blocked paths", func(t *testing.T) {
		// Arrange
		var buf strings.Builder

		// Act
		printPaths(logging.New(&buf), paths, true)

		// Assert
		assert.Contains(t, buf.String(), "agentic: proxy would block 2 request(s) to paths no rule allows")
	})

	t.Run("no blocked paths print nothing", func(t *testing.T) {
		// Arrange
		var buf strings.Builder

		// Act
		printPaths(logging.New(&buf), proxy.HostCount{}, false)

		// Assert
		assert.Empty(t, buf.String())
	})
}

func Test_printDenied(t *testing.T) {
	summary := proxy.Summary{Denied: proxy.HostCount{Hosts: []string{"evil.example.test"}, Count: 1}, Attempts: 2}

	t.Run("enforced mode reports blocked hosts", func(t *testing.T) {
		// Arrange
		var buf strings.Builder

		// Act
		printDenied(logging.New(&buf), summary, false)

		// Assert
		assert.Contains(t, buf.String(), "agentic: proxy blocked 1 request(s) to: evil.example.test")
		assert.Contains(t, buf.String(), "--no-proxy")
	})

	t.Run("monitor mode reports would-be-blocked hosts out of all attempts", func(t *testing.T) {
		// Arrange
		var buf strings.Builder

		// Act
		printDenied(logging.New(&buf), summary, true)

		// Assert
		assert.Contains(t, buf.String(), "agentic: proxy (monitor mode) observed 2 request(s); 1 would be blocked under the current allowlist: evil.example.test")
		assert.Contains(t, buf.String(), "--proxy-monitor")
	})

	t.Run("nothing denied prints nothing", func(t *testing.T) {
		// Arrange
		var buf strings.Builder

		// Act
		printDenied(logging.New(&buf), proxy.Summary{Attempts: 2}, true)

		// Assert
		assert.Empty(t, buf.String())
	})
}

func Test_printRefused(t *testing.T) {
	t.Run("reports hosts the address guard refused", func(t *testing.T) {
		// Arrange
		var buf strings.Builder

		// Act
		printRefused(logging.New(&buf), proxy.HostCount{Hosts: []string{"localhost"}, Count: 1})

		// Assert
		assert.Contains(t, buf.String(), "agentic: proxy refused 1 request(s) to local or private addresses: localhost")
	})

	t.Run("nothing refused prints nothing", func(t *testing.T) {
		// Arrange
		var buf strings.Builder

		// Act
		printRefused(logging.New(&buf), proxy.HostCount{})

		// Assert
		assert.Empty(t, buf.String())
	})
}

func Test_printMismatched(t *testing.T) {
	t.Run("warns about refused tunnels", func(t *testing.T) {
		// Arrange
		var buf strings.Builder

		// Act
		printMismatched(logging.New(&buf), proxy.HostCount{Hosts: []string{"cdn.example.test"}, Count: 1}, false)

		// Assert
		assert.Contains(t, buf.String(), "agentic: warning: proxy refused 1 tunnel(s) whose TLS server name wasn't the host they connected to: cdn.example.test")
	})

	t.Run("monitor mode says the tunnels were seen", func(t *testing.T) {
		// Arrange
		var buf strings.Builder

		// Act
		printMismatched(logging.New(&buf), proxy.HostCount{Hosts: []string{"cdn.example.test"}, Count: 1}, true)

		// Assert
		assert.Contains(t, buf.String(), "agentic: warning: proxy saw 1 tunnel(s)")
	})

	t.Run("nothing mismatched prints nothing", func(t *testing.T) {
		// Arrange
		var buf strings.Builder

		// Act
		printMismatched(logging.New(&buf), proxy.HostCount{}, false)

		// Assert
		assert.Empty(t, buf.String())
	})
}

func Test_printMissed(t *testing.T) {
	t.Run("warns about credential hosts reached without the placeholder", func(t *testing.T) {
		// Arrange
		var buf strings.Builder

		// Act
		printMissed(logging.New(&buf), proxy.HostCount{Hosts: []string{"api.example.test"}, Count: 1})

		// Assert
		assert.Contains(t, buf.String(), "agentic: warning: proxy saw 1 connection(s) to credential hosts without the placeholder, so they likely ran unauthenticated: api.example.test")
	})

	t.Run("nothing missed prints nothing", func(t *testing.T) {
		// Arrange
		var buf strings.Builder

		// Act
		printMissed(logging.New(&buf), proxy.HostCount{})

		// Assert
		assert.Empty(t, buf.String())
	})
}
