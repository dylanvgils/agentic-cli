package precedence

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config/rc"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyMode(t *testing.T) {
	enabled := true
	disabled := false
	creds := []rc.RCCredential{{Hosts: []string{"api.example.test"}, Header: "X-Api-Key", Secret: "/example/token"}}

	for _, tc := range []struct {
		name    string
		flags   ProxyInput
		proxy   rc.RCProxy
		injects bool
		want    sandbox.EgressMode
		wantErr string
	}{
		{name: "no flag and no config defaults off", injects: true, want: sandbox.EgressOff},
		{name: "config enabled is honored", proxy: rc.RCProxy{Enabled: &enabled}, injects: true, want: sandbox.EgressEnforce},
		{name: "proxy flag overrides config disabled", flags: ProxyInput{ProxyFlag: true}, proxy: rc.RCProxy{Enabled: &disabled}, injects: true, want: sandbox.EgressEnforce},
		{name: "no-proxy flag overrides config enabled", flags: ProxyInput{NoProxy: true}, proxy: rc.RCProxy{Enabled: &enabled}, injects: true, want: sandbox.EgressOff},
		{name: "proxy-monitor flag enables monitor mode", flags: ProxyInput{MonitorFlag: true}, injects: true, want: sandbox.EgressMonitor},
		{name: "no-proxy flag overrides proxy-monitor flag", flags: ProxyInput{MonitorFlag: true, NoProxy: true}, injects: true, want: sandbox.EgressOff},
		{name: "config mode monitor implies enabled", proxy: rc.RCProxy{Mode: rc.ModeMonitor}, injects: true, want: sandbox.EgressMonitor},
		{name: "config enabled false wins over config mode monitor", proxy: rc.RCProxy{Enabled: &disabled, Mode: rc.ModeMonitor}, injects: true, want: sandbox.EgressOff},
		{name: "credentials turn the proxy on", proxy: rc.RCProxy{Credentials: creds}, injects: true, want: sandbox.EgressEnforce},
		{name: "credentials keep monitor mode", flags: ProxyInput{MonitorFlag: true}, proxy: rc.RCProxy{Credentials: creds}, injects: true, want: sandbox.EgressMonitor},
		{name: "no-proxy flag with credentials is an error", flags: ProxyInput{NoProxy: true}, proxy: rc.RCProxy{Credentials: creds}, injects: true, wantErr: "--no-proxy cannot be used"},
		{name: "config enabled false with credentials is an error", proxy: rc.RCProxy{Enabled: &disabled, Credentials: creds}, injects: true, wantErr: "enabled = false cannot be used"},
		{name: "proxy flag overrides config disabled with credentials", flags: ProxyInput{ProxyFlag: true}, proxy: rc.RCProxy{Enabled: &disabled, Credentials: creds}, injects: true, want: sandbox.EgressEnforce},
		{name: "credentials leave the proxy off when it doesn't inject them", proxy: rc.RCProxy{Credentials: creds}, want: sandbox.EgressOff},
		{name: "no-proxy flag with credentials is allowed when the proxy doesn't inject them", flags: ProxyInput{NoProxy: true}, proxy: rc.RCProxy{Credentials: creds}, want: sandbox.EgressOff},
		{name: "config enabled false with credentials is allowed when the proxy doesn't inject them", proxy: rc.RCProxy{Enabled: &disabled, Credentials: creds}, want: sandbox.EgressOff},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			agenticRC := &rc.AgenticRC{Run: rc.RCRun{Proxy: tc.proxy}}

			// Act
			result, err := ProxyMode(tc.flags, agenticRC, tc.injects)

			// Assert
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestRequestedProxyMode(t *testing.T) {
	// Arrange
	agenticRC := &rc.AgenticRC{Run: rc.RCRun{Proxy: rc.RCProxy{Credentials: []rc.RCCredential{{Hosts: []string{"api.example.test"}}}}}}

	// Act
	result := RequestedProxyMode(ProxyInput{}, agenticRC)

	// Assert
	assert.Equal(t, sandbox.EgressOff, result)
}

func Test_proxyAllowList(t *testing.T) {
	t.Run("credential hosts come after rc hosts", func(t *testing.T) {
		// Arrange
		agenticRC := &rc.AgenticRC{}
		agenticRC.Run.Proxy.AllowedHosts = []string{"extra.example.com"}

		// Act
		result := proxyAllowList([]string{"api.example.com"}, []string{"api.custom.test"}, false, agenticRC)

		// Assert
		assert.Equal(t, []string{"api.example.com", "extra.example.com", "api.custom.test"}, result)
	})

	t.Run("dind adds docker hub between baseline and rc hosts", func(t *testing.T) {
		// Arrange
		agenticRC := &rc.AgenticRC{}
		agenticRC.Run.Proxy.AllowedHosts = []string{"ghcr.io"}

		// Act
		result := proxyAllowList([]string{"api.example.com"}, nil, true, agenticRC)

		// Assert
		assert.Equal(t, []string{"api.example.com", "registry-1.docker.io", "auth.docker.io", "production.cloudflare.docker.com", "production.cloudfront.docker.com", "ghcr.io"}, result)
	})

	t.Run("keeps the first of duplicate hosts", func(t *testing.T) {
		// Arrange
		agenticRC := &rc.AgenticRC{}
		agenticRC.Run.Proxy.AllowedHosts = []string{"extra.example.com", "api.example.com"}

		// Act
		result := proxyAllowList([]string{"api.example.com"}, nil, false, agenticRC)

		// Assert
		assert.Equal(t, []string{"api.example.com", "extra.example.com"}, result)
	})
}

func TestProxyBackend(t *testing.T) {
	agenticRC := &rc.AgenticRC{Run: rc.RCRun{Proxy: rc.RCProxy{Backend: rc.BackendAgentic}}}

	for _, tc := range []struct {
		name string
		in   ProxyInput
		rc   *rc.AgenticRC
		want string
	}{
		{
			name: "the flag wins over the config",
			in:   ProxyInput{Backend: rc.BackendNative},
			rc:   agenticRC,
			want: rc.BackendNative,
		},
		{
			name: "the config applies without the flag",
			rc:   agenticRC,
			want: rc.BackendAgentic,
		},
		{
			name: "neither set is empty",
			rc:   &rc.AgenticRC{},
			want: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			backend := ProxyBackend(tc.in, tc.rc)

			// Assert
			assert.Equal(t, tc.want, backend)
		})
	}
}

func TestProxyIntegrations(t *testing.T) {
	// Arrange
	agenticRC := &rc.AgenticRC{}
	agenticRC.Run.Proxy.Integrations = []string{"maven", "gradle"}
	in := ProxyInput{Integrations: []string{"maven"}}

	// Act
	result := ProxyIntegrations(in, agenticRC)

	// Assert
	assert.Equal(t, []string{"gradle", "maven"}, result)
}

func TestProxyRules(t *testing.T) {
	t.Run("allows merged hosts whole, then the path rules", func(t *testing.T) {
		// Arrange
		agenticRC := &rc.AgenticRC{}
		agenticRC.Run.Proxy.AllowedHosts = []string{"extra.example.test"}
		agenticRC.Run.Proxy.AllowedPaths = []rc.RCAllowedPath{{Host: "raw.example.test", Paths: []string{"/a/**"}}}

		// Act
		rules := ProxyRules([]string{"api.example.test"}, nil, false, agenticRC)

		// Assert
		assert.Equal(t, []sandbox.AllowRule{
			{Host: "api.example.test"},
			{Host: "extra.example.test"},
			{Host: "raw.example.test", Paths: []string{"/a/**"}},
		}, rules)
	})

	t.Run("path rules replace a host otherwise allowed whole", func(t *testing.T) {
		// Arrange
		agenticRC := &rc.AgenticRC{}
		agenticRC.Run.Proxy.AllowedHosts = []string{"Exact.Example.Test", ".wild.example.test"}
		agenticRC.Run.Proxy.AllowedPaths = []rc.RCAllowedPath{
			{Host: "exact.example.test", Paths: []string{"/a"}},
			{Host: "raw.wild.example.test", Paths: []string{"/a"}},
			{Host: "api.example.test", Paths: []string{"/repos/org/**"}},
		}

		// Act
		rules := ProxyRules(nil, []string{"api.example.test", "upload.example.test"}, false, agenticRC)

		// Assert
		assert.Equal(t, []sandbox.AllowRule{
			{Host: ".wild.example.test"},
			{Host: "upload.example.test"},
			{Host: "exact.example.test", Paths: []string{"/a"}},
			{Host: "raw.wild.example.test", Paths: []string{"/a"}},
			{Host: "api.example.test", Paths: []string{"/repos/org/**"}},
		}, rules)
	})
}

func Test_mergePathRules(t *testing.T) {
	// Arrange
	rules := []rc.RCAllowedPath{
		{Host: "raw.example.test", Paths: []string{"/a/**"}},
		{Host: "other.example.test", Paths: []string{"/b"}},
		{Host: "Raw.Example.Test.", Paths: []string{"/a/**", "/c"}},
	}

	// Act
	merged := mergePathRules(rules)

	// Assert
	assert.Equal(t, []sandbox.AllowRule{
		{Host: "raw.example.test", Paths: []string{"/a/**", "/c"}},
		{Host: "other.example.test", Paths: []string{"/b"}},
	}, merged)
}
