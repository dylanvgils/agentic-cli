package resolve

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyMode(t *testing.T) {
	enabled := true
	disabled := false
	creds := []config.RCCredential{{Hosts: []string{"api.example.test"}, Header: "X-Api-Key", Secret: "/example/token"}}

	t.Run("no flag and no config defaults off", func(t *testing.T) {
		// Act
		result, err := ProxyMode(ProxyInput{}, &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, docker.ProxyOff, result)
	})

	t.Run("config enabled is honored", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &enabled}}}

		// Act
		result, err := ProxyMode(ProxyInput{}, rc)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, docker.ProxyEnforce, result)
	})

	t.Run("proxy flag overrides config disabled", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &disabled}}}

		// Act
		result, err := ProxyMode(ProxyInput{ProxyFlag: true}, rc)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, docker.ProxyEnforce, result)
	})

	t.Run("no-proxy flag overrides config enabled", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &enabled}}}

		// Act
		result, err := ProxyMode(ProxyInput{NoProxy: true}, rc)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, docker.ProxyOff, result)
	})

	t.Run("proxy-monitor flag enables monitor mode", func(t *testing.T) {
		// Act
		result, err := ProxyMode(ProxyInput{MonitorFlag: true}, &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, docker.ProxyMonitor, result)
	})

	t.Run("no-proxy flag overrides proxy-monitor flag", func(t *testing.T) {
		// Act
		result, err := ProxyMode(ProxyInput{MonitorFlag: true, NoProxy: true}, &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, docker.ProxyOff, result)
	})

	t.Run("config mode monitor implies enabled", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Mode: config.ModeMonitor}}}

		// Act
		result, err := ProxyMode(ProxyInput{}, rc)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, docker.ProxyMonitor, result)
	})

	t.Run("config enabled false wins over config mode monitor", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &disabled, Mode: config.ModeMonitor}}}

		// Act
		result, err := ProxyMode(ProxyInput{}, rc)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, docker.ProxyOff, result)
	})

	t.Run("credentials turn the proxy on", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Credentials: creds}}}

		// Act
		result, err := ProxyMode(ProxyInput{}, rc)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, docker.ProxyEnforce, result)
	})

	t.Run("credentials keep monitor mode", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Credentials: creds}}}

		// Act
		result, err := ProxyMode(ProxyInput{MonitorFlag: true}, rc)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, docker.ProxyMonitor, result)
	})

	t.Run("no-proxy flag with credentials is an error", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Credentials: creds}}}

		// Act
		_, err := ProxyMode(ProxyInput{NoProxy: true}, rc)

		// Assert
		assert.ErrorContains(t, err, "--no-proxy cannot be used")
	})

	t.Run("config enabled false with credentials is an error", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &disabled, Credentials: creds}}}

		// Act
		_, err := ProxyMode(ProxyInput{}, rc)

		// Assert
		assert.ErrorContains(t, err, "enabled = false cannot be used")
	})

	t.Run("proxy flag overrides config disabled with credentials", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &disabled, Credentials: creds}}}

		// Act
		result, err := ProxyMode(ProxyInput{ProxyFlag: true}, rc)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, docker.ProxyEnforce, result)
	})
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
		assert.Equal(t, []string{"api.example.com", "registry-1.docker.io", "auth.docker.io", "production.cloudflare.docker.com", "ghcr.io"}, result)
	})

	t.Run("empty allowlists on both sides return empty result", func(t *testing.T) {
		// Act
		result := ProxyAllowList(nil, false, &config.AgenticRC{})

		// Assert
		assert.Empty(t, result)
	})
}
