package run

import (
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
)

func Test_printProxySummary(t *testing.T) {
	t.Run("sets what the log flagged apart from the tool output", func(t *testing.T) {
		// Arrange
		logFile := writeProxyLog(t,
			`{"host":"evil.example.test","decision":"deny"}`,
			`{"host":"localhost","decision":"deny","reason":"blocked-address"}`,
			`{"host":"api.example.test","decision":"allow","intercepted":true,"reason":"no-placeholder"}`,
		)
		buf := iostreamstest.StubErr(t)

		// Act
		printProxySummary(logFile, sandbox.EgressEnforce)

		// Assert
		assert.True(t, strings.HasPrefix(buf.String(), "\nagentic: proxy blocked 1 request(s) to: evil.example.test"))
		assert.Contains(t, buf.String(), "proxy refused 1 request(s)")
		assert.Contains(t, buf.String(), "without the placeholder")
	})

	t.Run("nothing flagged prints nothing", func(t *testing.T) {
		// Arrange
		logFile := writeProxyLog(t, `{"host":"api.anthropic.com","decision":"allow"}`)
		buf := iostreamstest.StubErr(t)

		// Act
		printProxySummary(logFile, sandbox.EgressMonitor)

		// Assert
		assert.Empty(t, buf.String())
	})
}

func Test_printFlagged(t *testing.T) {
	paths := proxy.HostCount{Hosts: []string{"raw.example.test/other"}, Count: 2}
	summary := proxy.Summary{Denied: proxy.HostCount{Hosts: []string{"evil.example.test"}, Count: 1}, Attempts: 2}
	mismatched := proxy.HostCount{Hosts: []string{"cdn.example.test"}, Count: 1}

	for _, tc := range []struct {
		name  string
		print func()
		want  string
	}{
		{
			name:  "enforced mode reports blocked paths",
			print: func() { printPaths(paths, false) },
			want: "agentic: proxy blocked 2 request(s) to paths no rule allows: raw.example.test/other\n" +
				"   add the ones you want to allow to their host's paths in [[run.proxy.allowed_paths]].\n",
		},
		{
			name:  "monitor mode reports would-be-blocked paths",
			print: func() { printPaths(paths, true) },
			want: "agentic: proxy would block 2 request(s) to paths no rule allows: raw.example.test/other\n" +
				"   add the ones you want to allow to their host's paths in [[run.proxy.allowed_paths]].\n",
		},
		{
			name:  "no blocked paths print nothing",
			print: func() { printPaths(proxy.HostCount{}, false) },
		},
		{
			name:  "enforced mode reports blocked hosts",
			print: func() { printDenied(summary, false) },
			want: "agentic: proxy blocked 1 request(s) to: evil.example.test\n" +
				"   add them to [run.proxy] allowed_hosts (or pass --no-proxy) to permit.\n",
		},
		{
			name:  "monitor mode reports would-be-blocked hosts out of all attempts",
			print: func() { printDenied(summary, true) },
			want: "agentic: proxy (monitor mode) observed 2 request(s); 1 would be blocked under the current allowlist: evil.example.test\n" +
				"   add the ones you want to allow to [run.proxy] allowed_hosts, then drop --proxy-monitor.\n",
		},
		{
			name:  "nothing denied prints nothing",
			print: func() { printDenied(proxy.Summary{Attempts: 2}, true) },
		},
		{
			name:  "reports hosts the address guard refused",
			print: func() { printRefused(proxy.HostCount{Hosts: []string{"localhost"}, Count: 1}) },
			want: "agentic: proxy refused 1 request(s) to local or private addresses: localhost\n" +
				"   list an internal host exactly in [run.proxy] allowed_hosts to permit its private address; loopback and link-local stay refused.\n",
		},
		{
			name:  "nothing refused prints nothing",
			print: func() { printRefused(proxy.HostCount{}) },
		},
		{
			name:  "warns about refused tunnels",
			print: func() { printMismatched(mismatched, false) },
			want: "agentic: warning: proxy refused 1 tunnel(s) whose TLS server name wasn't the host they connected to: cdn.example.test\n" +
				"   the tool may be reaching a site that isn't allowlisted through an allowed host; tunnels must carry TLS for that host.\n",
		},
		{
			name:  "monitor mode says the tunnels were seen",
			print: func() { printMismatched(mismatched, true) },
			want: "agentic: warning: proxy saw 1 tunnel(s) whose TLS server name wasn't the host they connected to: cdn.example.test\n" +
				"   the tool may be reaching a site that isn't allowlisted through an allowed host; tunnels must carry TLS for that host.\n",
		},
		{
			name:  "nothing mismatched prints nothing",
			print: func() { printMismatched(proxy.HostCount{}, false) },
		},
		{
			name:  "warns about credential hosts reached without the placeholder",
			print: func() { printMissed(proxy.HostCount{Hosts: []string{"api.example.test"}, Count: 1}) },
			want: "agentic: warning: proxy saw 1 connection(s) to credential hosts without the placeholder, so they likely ran unauthenticated: api.example.test\n" +
				"   check that the tool reads one of the credential's env names and sends it in a listed header.\n",
		},
		{
			name:  "nothing missed prints nothing",
			print: func() { printMissed(proxy.HostCount{}) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			buf := iostreamstest.StubErr(t)

			// Act
			tc.print()

			// Assert
			assert.Equal(t, tc.want, buf.String())
		})
	}
}
