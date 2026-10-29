package resolve

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyMode(t *testing.T) {
	enabled := true
	disabled := false
	creds := []config.RCCredential{{Hosts: []string{"api.example.test"}, Header: "X-Api-Key", Secret: "/example/token"}}

	t.Run("no flag and no config defaults off", func(t *testing.T) {
		// Act
		result, err := ProxyMode(ProxyInput{}, &config.AgenticRC{}, true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ProxyOff, result)
	})

	t.Run("config enabled is honored", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &enabled}}}

		// Act
		result, err := ProxyMode(ProxyInput{}, rc, true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ProxyEnforce, result)
	})

	t.Run("proxy flag overrides config disabled", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &disabled}}}

		// Act
		result, err := ProxyMode(ProxyInput{ProxyFlag: true}, rc, true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ProxyEnforce, result)
	})

	t.Run("no-proxy flag overrides config enabled", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &enabled}}}

		// Act
		result, err := ProxyMode(ProxyInput{NoProxy: true}, rc, true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ProxyOff, result)
	})

	t.Run("proxy-monitor flag enables monitor mode", func(t *testing.T) {
		// Act
		result, err := ProxyMode(ProxyInput{MonitorFlag: true}, &config.AgenticRC{}, true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ProxyMonitor, result)
	})

	t.Run("no-proxy flag overrides proxy-monitor flag", func(t *testing.T) {
		// Act
		result, err := ProxyMode(ProxyInput{MonitorFlag: true, NoProxy: true}, &config.AgenticRC{}, true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ProxyOff, result)
	})

	t.Run("config mode monitor implies enabled", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Mode: config.ModeMonitor}}}

		// Act
		result, err := ProxyMode(ProxyInput{}, rc, true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ProxyMonitor, result)
	})

	t.Run("config enabled false wins over config mode monitor", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &disabled, Mode: config.ModeMonitor}}}

		// Act
		result, err := ProxyMode(ProxyInput{}, rc, true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ProxyOff, result)
	})

	t.Run("credentials turn the proxy on", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Credentials: creds}}}

		// Act
		result, err := ProxyMode(ProxyInput{}, rc, true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ProxyEnforce, result)
	})

	t.Run("credentials keep monitor mode", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Credentials: creds}}}

		// Act
		result, err := ProxyMode(ProxyInput{MonitorFlag: true}, rc, true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ProxyMonitor, result)
	})

	t.Run("no-proxy flag with credentials is an error", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Credentials: creds}}}

		// Act
		_, err := ProxyMode(ProxyInput{NoProxy: true}, rc, true)

		// Assert
		assert.ErrorContains(t, err, "--no-proxy cannot be used")
	})

	t.Run("config enabled false with credentials is an error", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &disabled, Credentials: creds}}}

		// Act
		_, err := ProxyMode(ProxyInput{}, rc, true)

		// Assert
		assert.ErrorContains(t, err, "enabled = false cannot be used")
	})

	t.Run("proxy flag overrides config disabled with credentials", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &disabled, Credentials: creds}}}

		// Act
		result, err := ProxyMode(ProxyInput{ProxyFlag: true}, rc, true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ProxyEnforce, result)
	})

	t.Run("credentials leave the proxy off when it doesn't inject them", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Credentials: creds}}}

		// Act
		result, err := ProxyMode(ProxyInput{}, rc, false)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ProxyOff, result)
	})

	t.Run("no-proxy flag with credentials is allowed when the proxy doesn't inject them", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Credentials: creds}}}

		// Act
		result, err := ProxyMode(ProxyInput{NoProxy: true}, rc, false)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ProxyOff, result)
	})

	t.Run("config enabled false with credentials is allowed when the proxy doesn't inject them", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &disabled, Credentials: creds}}}

		// Act
		result, err := ProxyMode(ProxyInput{}, rc, false)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ProxyOff, result)
	})
}

func TestRequestedProxyMode(t *testing.T) {
	// Arrange
	rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Credentials: []config.RCCredential{{Hosts: []string{"api.example.test"}}}}}}

	// Act
	result := RequestedProxyMode(ProxyInput{}, rc)

	// Assert
	assert.Equal(t, sandbox.ProxyOff, result)
}

func TestProxyAllowList(t *testing.T) {
	t.Run("credential hosts come after rc hosts", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.AllowedHosts = []string{"extra.example.com"}
		rc.Run.Proxy.Credentials = []config.RCCredential{{Hosts: []string{"api.custom.test"}, Header: "X-Api-Key", Secret: "/example/token"}}

		// Act
		result := ProxyAllowList([]string{"api.example.com"}, false, rc)

		// Assert
		assert.Equal(t, []string{"api.example.com", "extra.example.com", "api.custom.test"}, result)
	})

	t.Run("merges tool baseline with rc-configured hosts", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.AllowedHosts = []string{"extra.example.com"}

		// Act
		result := ProxyAllowList([]string{"api.example.com"}, false, rc)

		// Assert
		assert.Equal(t, []string{"api.example.com", "extra.example.com"}, result)
	})

	t.Run("dind adds docker hub between baseline and rc hosts", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.AllowedHosts = []string{"ghcr.io"}

		// Act
		result := ProxyAllowList([]string{"api.example.com"}, true, rc)

		// Assert
		assert.Equal(t, []string{"api.example.com", "registry-1.docker.io", "auth.docker.io", "production.cloudflare.docker.com", "production.cloudfront.docker.com", "ghcr.io"}, result)
	})

	t.Run("keeps the first of duplicate hosts", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.AllowedHosts = []string{"extra.example.com", "api.example.com"}

		// Act
		result := ProxyAllowList([]string{"api.example.com"}, false, rc)

		// Assert
		assert.Equal(t, []string{"api.example.com", "extra.example.com"}, result)
	})

	t.Run("empty allowlists on both sides return empty result", func(t *testing.T) {
		// Act
		result := ProxyAllowList(nil, false, &config.AgenticRC{})

		// Assert
		assert.Empty(t, result)
	})
}

func TestProxyIntegrations(t *testing.T) {
	// Arrange
	rc := &config.AgenticRC{}
	rc.Run.Proxy.Integrations = []string{"maven", "gradle"}
	in := ProxyInput{Integrations: []string{"maven"}}

	// Act
	result := ProxyIntegrations(in, rc)

	// Assert
	assert.Equal(t, []string{"gradle", "maven"}, result)
}

func TestProxyRules(t *testing.T) {
	t.Run("allows merged hosts whole, then the path rules", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.AllowedHosts = []string{"extra.example.test"}
		rc.Run.Proxy.AllowedPaths = []config.RCAllowedPath{{Host: "raw.example.test", Paths: []string{"/a/**"}}}

		// Act
		rules := ProxyRules([]string{"api.example.test"}, false, rc)

		// Assert
		assert.Equal(t, []sandbox.AllowRule{
			{Host: "api.example.test"},
			{Host: "extra.example.test"},
			{Host: "raw.example.test", Paths: []string{"/a/**"}},
		}, rules)
	})

	t.Run("path rules replace a host otherwise allowed whole", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.AllowedHosts = []string{"Exact.Example.Test", ".wild.example.test"}
		rc.Run.Proxy.Credentials = []config.RCCredential{{Hosts: []string{"api.example.test", "upload.example.test"}, Header: "Authorization", Secret: "/example/token"}}
		rc.Run.Proxy.AllowedPaths = []config.RCAllowedPath{
			{Host: "exact.example.test", Paths: []string{"/a"}},
			{Host: "raw.wild.example.test", Paths: []string{"/a"}},
			{Host: "api.example.test", Paths: []string{"/repos/org/**"}},
		}

		// Act
		rules := ProxyRules(nil, false, rc)

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
	rules := []config.RCAllowedPath{
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
