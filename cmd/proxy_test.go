package cmd

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_resolveProxyEnabled(t *testing.T) {
	enabled := true
	disabled := false

	t.Run("no flag and no config defaults off", func(t *testing.T) {
		// Act
		result := resolveProxyEnabled(runToolCmd, &config.AgenticRC{})

		// Assert
		assert.False(t, result)
	})

	t.Run("config enabled is honored", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &enabled}}}

		// Act
		result := resolveProxyEnabled(runToolCmd, rc)

		// Assert
		assert.True(t, result)
	})

	t.Run("proxy flag overrides config disabled", func(t *testing.T) {
		// Arrange
		require.NoError(t, runToolCmd.Flags().Set("proxy", "true"))
		t.Cleanup(func() {
			_ = runToolCmd.Flags().Set("proxy", "false")
			runToolCmd.Flags().Lookup("proxy").Changed = false
		})
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &disabled}}}

		// Act
		result := resolveProxyEnabled(runToolCmd, rc)

		// Assert
		assert.True(t, result)
	})

	t.Run("no-proxy flag overrides config enabled", func(t *testing.T) {
		// Arrange
		require.NoError(t, runToolCmd.Flags().Set("no-proxy", "true"))
		t.Cleanup(func() {
			_ = runToolCmd.Flags().Set("no-proxy", "false")
			runToolCmd.Flags().Lookup("no-proxy").Changed = false
		})
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &enabled}}}

		// Act
		result := resolveProxyEnabled(runToolCmd, rc)

		// Assert
		assert.False(t, result)
	})
}

func Test_ensureProxyImage(t *testing.T) {
	t.Run("builds the image when missing", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)
		var built string
		stubBuildProxyImage(t, func(image, _, _ string, _ tools.BuildOptions) error {
			built = image
			return nil
		})

		// Act
		err := ensureProxyImage(runToolCmd, "myns")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "myns-proxy", built)
	})

	t.Run("skips build when image already exists", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{Image: "myns-proxy"}, nil)
		built := false
		stubBuildProxyImage(t, func(string, string, string, tools.BuildOptions) error {
			built = true
			return nil
		})

		// Act
		err := ensureProxyImage(runToolCmd, "myns")

		// Assert
		require.NoError(t, err)
		assert.False(t, built)
	})
}
