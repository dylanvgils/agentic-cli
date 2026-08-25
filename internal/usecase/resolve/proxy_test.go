package resolve

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestProxyMode(t *testing.T) {
	enabled := true
	disabled := false

	t.Run("no flag and no config defaults off", func(t *testing.T) {
		// Act
		gotEnabled, gotMonitor := ProxyMode(ProxyInput{}, &config.AgenticRC{})

		// Assert
		assert.False(t, gotEnabled)
		assert.False(t, gotMonitor)
	})

	t.Run("config enabled is honored", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &enabled}}}

		// Act
		gotEnabled, gotMonitor := ProxyMode(ProxyInput{}, rc)

		// Assert
		assert.True(t, gotEnabled)
		assert.False(t, gotMonitor)
	})

	t.Run("proxy flag overrides config disabled", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &disabled}}}

		// Act
		gotEnabled, gotMonitor := ProxyMode(ProxyInput{ProxyFlag: true}, rc)

		// Assert
		assert.True(t, gotEnabled)
		assert.False(t, gotMonitor)
	})

	t.Run("no-proxy flag overrides config enabled", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &enabled}}}

		// Act
		gotEnabled, gotMonitor := ProxyMode(ProxyInput{NoProxy: true}, rc)

		// Assert
		assert.False(t, gotEnabled)
		assert.False(t, gotMonitor)
	})

	t.Run("proxy-monitor flag enables monitor mode", func(t *testing.T) {
		// Act
		gotEnabled, gotMonitor := ProxyMode(ProxyInput{MonitorFlag: true}, &config.AgenticRC{})

		// Assert
		assert.True(t, gotEnabled)
		assert.True(t, gotMonitor)
	})

	t.Run("no-proxy flag overrides proxy-monitor flag", func(t *testing.T) {
		// Act
		gotEnabled, gotMonitor := ProxyMode(ProxyInput{MonitorFlag: true, NoProxy: true}, &config.AgenticRC{})

		// Assert
		assert.False(t, gotEnabled)
		assert.False(t, gotMonitor)
	})

	t.Run("config mode monitor implies enabled", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Mode: config.ModeMonitor}}}

		// Act
		gotEnabled, gotMonitor := ProxyMode(ProxyInput{}, rc)

		// Assert
		assert.True(t, gotEnabled)
		assert.True(t, gotMonitor)
	})

	t.Run("config enabled false wins over config mode monitor", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &disabled, Mode: config.ModeMonitor}}}

		// Act
		gotEnabled, gotMonitor := ProxyMode(ProxyInput{}, rc)

		// Assert
		assert.False(t, gotEnabled)
		assert.False(t, gotMonitor)
	})
}

func TestProxyAllowList(t *testing.T) {
	t.Run("merges tool baseline with rc-configured hosts", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.AllowedHosts = []string{"extra.example.com"}

		// Act
		result := ProxyAllowList([]string{"api.example.com"}, rc)

		// Assert
		assert.Equal(t, []string{"api.example.com", "extra.example.com"}, result)
	})

	t.Run("empty allowlists on both sides return empty result", func(t *testing.T) {
		// Act
		result := ProxyAllowList(nil, &config.AgenticRC{})

		// Assert
		assert.Empty(t, result)
	})
}
