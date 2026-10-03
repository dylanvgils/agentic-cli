package resolve

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestDindEnabled(t *testing.T) {
	enabled := true

	t.Run("no flag and no config defaults off", func(t *testing.T) {
		// Act
		result := DindEnabled(DindInput{}, &config.AgenticRC{})

		// Assert
		assert.False(t, result)
	})

	t.Run("config enabled is honored", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Dind: config.RCDind{Enabled: &enabled}}}

		// Act
		result := DindEnabled(DindInput{}, rc)

		// Assert
		assert.True(t, result)
	})

	t.Run("no-dind flag overrides config enabled", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Dind: config.RCDind{Enabled: &enabled}}}

		// Act
		result := DindEnabled(DindInput{NoDindFlag: true}, rc)

		// Assert
		assert.False(t, result)
	})
}
