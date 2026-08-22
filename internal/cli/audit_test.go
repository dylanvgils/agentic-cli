package cli

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_resolveAuditEnabled(t *testing.T) {
	enabled := true
	disabled := false

	t.Run("no flag and no config defaults off", func(t *testing.T) {
		// Act
		got := resolveAuditEnabled(runToolCmd, &config.AgenticRC{})

		// Assert
		assert.False(t, got)
	})

	t.Run("config enabled is honored", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Audit: config.RCAudit{Enabled: &enabled}}}

		// Act
		got := resolveAuditEnabled(runToolCmd, rc)

		// Assert
		assert.True(t, got)
	})

	t.Run("audit flag overrides config disabled", func(t *testing.T) {
		// Arrange
		require.NoError(t, runToolCmd.Flags().Set("audit", "true"))
		t.Cleanup(func() {
			_ = runToolCmd.Flags().Set("audit", "false")
			runToolCmd.Flags().Lookup("audit").Changed = false
		})
		rc := &config.AgenticRC{Run: config.RCRun{Audit: config.RCAudit{Enabled: &disabled}}}

		// Act
		got := resolveAuditEnabled(runToolCmd, rc)

		// Assert
		assert.True(t, got)
	})

	t.Run("no-audit flag overrides config enabled", func(t *testing.T) {
		// Arrange
		require.NoError(t, runToolCmd.Flags().Set("no-audit", "true"))
		t.Cleanup(func() {
			_ = runToolCmd.Flags().Set("no-audit", "false")
			runToolCmd.Flags().Lookup("no-audit").Changed = false
		})
		rc := &config.AgenticRC{Run: config.RCRun{Audit: config.RCAudit{Enabled: &enabled}}}

		// Act
		got := resolveAuditEnabled(runToolCmd, rc)

		// Assert
		assert.False(t, got)
	})
}
