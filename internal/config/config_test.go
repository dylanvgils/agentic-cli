package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	t.Run("file not exist returns empty", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()

		// Act
		cfg, err := LoadConfig(dir)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, cfg.Registry)
	})

	t.Run("valid file returns parsed", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "agentic.json"),
			[]byte(`{"docker_context":"example","proxy_log_retention_days":7}`),
			0o640,
		))

		// Act
		cfg, err := LoadConfig(dir)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "example", cfg.DockerContext)
		assert.Equal(t, 7, cfg.ProxyLogRetentionDays)
	})

	t.Run("registry field is loaded", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "agentic.json"),
			[]byte(`{"registry":"myregistry.example.com"}`),
			0o640,
		))

		// Act
		cfg, err := LoadConfig(dir)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "myregistry.example.com", cfg.Registry)
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "agentic.json"),
			[]byte(`not-json`),
			0o640,
		))

		// Act
		_, err := LoadConfig(dir)

		// Assert
		require.Error(t, err)
	})
}

func TestConfigFile(t *testing.T) {
	// Act
	path := ConfigFile("/example.test/agentic")

	// Assert
	assert.Equal(t, filepath.Join("/example.test/agentic", "agentic.json"), path)
}
