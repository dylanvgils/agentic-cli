package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_resolvePrefix(t *testing.T) {
	t.Run("rc value wins over env", func(t *testing.T) {
		// Arrange
		t.Setenv(EnvPrefix, "fromenv")
		rc := &AgenticRC{Prefix: "fromrc"}

		// Act
		result := ResolvePrefix("", rc)

		// Assert
		assert.Equal(t, "fromrc", result)
	})

	t.Run("rc value used when flag and env absent", func(t *testing.T) {
		// Arrange
		rc := &AgenticRC{Prefix: "fromrc"}

		// Act
		result := ResolvePrefix("", rc)

		// Assert
		assert.Equal(t, "fromrc", result)
	})

	t.Run("falls back to default when nothing set", func(t *testing.T) {
		// Act
		result := ResolvePrefix("", nil)

		// Assert
		assert.Equal(t, DefaultPrefix, result)
	})
}

func Test_flagOrEnv(t *testing.T) {
	t.Run("flag takes priority", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_NODE_VERSION", "fromenv")

		// Act
		result := FlagOrEnv("fromflag", "AGENTIC_NODE_VERSION")

		// Assert
		assert.Equal(t, "fromflag", result)
	})

	t.Run("falls back to env when flag empty", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_NODE_VERSION", "fromenv")

		// Act
		result := FlagOrEnv("", "AGENTIC_NODE_VERSION")

		// Assert
		assert.Equal(t, "fromenv", result)
	})

	t.Run("returns empty when both unset", func(t *testing.T) {
		// Act + Assert
		assert.Equal(t, "", FlagOrEnv("", "AGENTIC_NODE_VERSION"))
	})
}

func Test_resolveRegistry(t *testing.T) {
	t.Run("flag takes priority over config", func(t *testing.T) {
		// Arrange
		homeDir := t.TempDir()
		cfg := &CliConfig{Registry: "config.example.com"}
		require.NoError(t, cfg.Save(homeDir))

		// Act
		result := ResolveRegistry("flag.example.com", homeDir)

		// Assert
		assert.Equal(t, "flag.example.com", result)
	})

	t.Run("falls back to agentic.json when flag not set", func(t *testing.T) {
		// Arrange
		homeDir := t.TempDir()
		cfg := &CliConfig{Registry: "config.example.com"}
		require.NoError(t, cfg.Save(homeDir))

		// Act
		result := ResolveRegistry("", homeDir)

		// Assert
		assert.Equal(t, "config.example.com", result)
	})

	t.Run("empty when neither set", func(t *testing.T) {
		// Act
		result := ResolveRegistry("", t.TempDir())

		// Assert
		assert.Empty(t, result)
	})

	t.Run("empty when homeDir has no config file", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "agentic.json"), []byte("invalid json"), 0o644))

		// Act
		result := ResolveRegistry("", dir)

		// Assert
		assert.Empty(t, result)
	})
}
