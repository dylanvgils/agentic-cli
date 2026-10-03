package resolve

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
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

func TestDindResourceLimitsFor(t *testing.T) {
	tool := docker.ResourceLimits{PidsLimit: "1024", CPUs: "4", Memory: "4g"}

	t.Run("inherits tool limits when nothing set", func(t *testing.T) {
		// Act
		result := DindResourceLimitsFor(docker.ResourceLimits{}, &config.AgenticRC{}, tool)

		// Assert
		assert.Equal(t, tool, result)
	})

	t.Run("rc takes precedence over tool limits", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Dind: config.RCDind{PidsLimit: "2048", Memory: "8g"}}}

		// Act
		result := DindResourceLimitsFor(docker.ResourceLimits{}, rc, tool)

		// Assert
		assert.Equal(t, docker.ResourceLimits{PidsLimit: "2048", CPUs: "4", Memory: "8g"}, result)
	})

	t.Run("flag takes precedence over rc", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Dind: config.RCDind{PidsLimit: "2048", CPUs: "2", Memory: "8g"}}}

		// Act
		result := DindResourceLimitsFor(docker.ResourceLimits{CPUs: "1", Memory: "16g"}, rc, tool)

		// Assert
		assert.Equal(t, docker.ResourceLimits{PidsLimit: "2048", CPUs: "1", Memory: "16g"}, result)
	})
}
