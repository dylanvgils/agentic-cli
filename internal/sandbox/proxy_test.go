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

func TestProxySpec_Hosts(t *testing.T) {
	// Arrange
	spec := ProxySpec{Rules: []AllowRule{{Host: "api.example.test"}, {Host: ".example.org"}}}

	// Act
	hosts := spec.Hosts()

	// Assert
	assert.Equal(t, []string{"api.example.test", ".example.org"}, hosts)
}
