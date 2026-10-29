package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProxyMode_Enabled(t *testing.T) {
	t.Run("zero value is off", func(t *testing.T) {
		// Arrange
		var mode ProxyMode

		// Act
		result := mode.Enabled()

		// Assert
		assert.Equal(t, ProxyOff, mode)
		assert.False(t, result)
	})

	t.Run("enforce is enabled", func(t *testing.T) {
		// Act
		result := ProxyEnforce.Enabled()

		// Assert
		assert.True(t, result)
	})

	t.Run("monitor is enabled", func(t *testing.T) {
		// Act
		result := ProxyMonitor.Enabled()

		// Assert
		assert.True(t, result)
	})
}

func TestProxySpec_HasPathRules(t *testing.T) {
	t.Run("whole-host rules have none", func(t *testing.T) {
		// Arrange
		spec := ProxySpec{Rules: []AllowRule{{Host: "api.example.test"}}}

		// Act
		result := spec.HasPathRules()

		// Assert
		assert.False(t, result)
	})

	t.Run("a rule with paths counts", func(t *testing.T) {
		// Arrange
		spec := ProxySpec{Rules: []AllowRule{{Host: "api.example.test"}, {Host: "raw.example.test", Paths: []string{"/a"}}}}

		// Act
		result := spec.HasPathRules()

		// Assert
		assert.True(t, result)
	})
}

func TestAllowWhole(t *testing.T) {
	// Arrange
	rules := []AllowRule{{Host: "api.example.test"}, {Host: "raw.example.test", Paths: []string{"/a"}}}

	// Act
	whole := AllowWhole(rules)

	// Assert
	assert.Equal(t, []AllowRule{{Host: "api.example.test"}, {Host: "raw.example.test"}}, whole)
	assert.Equal(t, []string{"/a"}, rules[1].Paths, "the input is left as is")
}

func TestProxySpec_WholeHosts(t *testing.T) {
	// Arrange
	spec := ProxySpec{Rules: []AllowRule{{Host: "api.example.test"}, {Host: "raw.example.test", Paths: []string{"/a"}}, {Host: ".example.org"}}}

	// Act
	hosts := spec.WholeHosts()

	// Assert
	assert.Equal(t, []string{"api.example.test", ".example.org"}, hosts)
}

func TestProxyIntegrations(t *testing.T) {
	// Act
	names := ProxyIntegrations()

	// Assert
	assert.Equal(t, []string{"gradle", "maven"}, names)
}

func TestCheckProxyIntegrations(t *testing.T) {
	t.Run("known names", func(t *testing.T) {
		// Act
		err := CheckProxyIntegrations([]string{"maven", "gradle"})

		// Assert
		assert.NoError(t, err)
	})

	t.Run("unknown name returns error", func(t *testing.T) {
		// Act
		err := CheckProxyIntegrations([]string{"maven", "example"})

		// Assert
		assert.EqualError(t, err, `unknown integration "example": must be one of gradle, maven`)
	})
}

func TestProxyIntegrationEnv(t *testing.T) {
	t.Run("maven points at the global settings", func(t *testing.T) {
		// Act
		env := ProxyIntegrationEnv([]string{"maven"})

		// Assert
		assert.Equal(t, []string{"MAVEN_ARGS=--global-settings " + MavenSettingsPath}, env)
	})

	t.Run("gradle sets the proxy system properties", func(t *testing.T) {
		// Act
		env := ProxyIntegrationEnv([]string{"gradle"})

		// Assert
		assert.Equal(t, []string{"GRADLE_OPTS=-Dhttp.proxyHost=agentic-proxy -Dhttp.proxyPort=3128 -Dhttps.proxyHost=agentic-proxy -Dhttps.proxyPort=3128"}, env)
	})

	t.Run("none for no integrations", func(t *testing.T) {
		// Act
		env := ProxyIntegrationEnv(nil)

		// Assert
		assert.Empty(t, env)
	})
}

func TestProxyIntegrationEnvNames(t *testing.T) {
	// Act
	names := ProxyIntegrationEnvNames([]string{"gradle", "maven"})

	// Assert
	assert.Equal(t, []string{"GRADLE_OPTS", "MAVEN_ARGS"}, names)
}
