package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceLimits_Or(t *testing.T) {
	fallback := ResourceLimits{PidsLimit: "1024", CPUs: "4", Memory: "4g"}

	t.Run("empty limits take the fallback", func(t *testing.T) {
		// Act
		result := ResourceLimits{}.Or(fallback)

		// Assert
		assert.Equal(t, fallback, result)
	})

	t.Run("set limits are kept", func(t *testing.T) {
		// Arrange
		limits := ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}

		// Act
		result := limits.Or(fallback)

		// Assert
		assert.Equal(t, limits, result)
	})

	t.Run("each limit falls back on its own", func(t *testing.T) {
		// Arrange
		limits := ResourceLimits{CPUs: "2"}

		// Act
		result := limits.Or(fallback)

		// Assert
		assert.Equal(t, ResourceLimits{PidsLimit: "1024", CPUs: "2", Memory: "4g"}, result)
	})
}
