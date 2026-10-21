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
