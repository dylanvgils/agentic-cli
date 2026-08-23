package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_resolveNamespace(t *testing.T) {
	t.Run("flag takes priority over rc", func(t *testing.T) {
		// Arrange
		rc := &AgenticRC{Namespace: "fromrc"}

		// Act
		result := ResolveNamespace("fromflag", rc)

		// Assert
		assert.Equal(t, "fromflag", result)
	})

	t.Run("rc value used when flag absent", func(t *testing.T) {
		// Arrange
		rc := &AgenticRC{Namespace: "fromrc"}

		// Act
		result := ResolveNamespace("", rc)

		// Assert
		assert.Equal(t, "fromrc", result)
	})

	t.Run("falls back to default when nothing set", func(t *testing.T) {
		// Act
		result := ResolveNamespace("", nil)

		// Assert
		assert.Equal(t, DefaultNamespace, result)
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

func Test_resolveDockerContext(t *testing.T) {
	t.Run("flag wins over everything", func(t *testing.T) {
		// Arrange
		homeDir := t.TempDir()
		require.NoError(t, (&CliConfig{DockerContext: "fromconfig"}).Save(homeDir))
		rc := &AgenticRC{DockerContext: "fromrc"}

		// Act
		result := ResolveDockerContext("fromflag", rc, homeDir)

		// Assert
		assert.Equal(t, "fromflag", result)
	})

	t.Run("rc value wins over agentic.json", func(t *testing.T) {
		// Arrange
		homeDir := t.TempDir()
		require.NoError(t, (&CliConfig{DockerContext: "fromconfig"}).Save(homeDir))
		rc := &AgenticRC{DockerContext: "fromrc"}

		// Act
		result := ResolveDockerContext("", rc, homeDir)

		// Assert
		assert.Equal(t, "fromrc", result)
	})

	t.Run("agentic.json wins when rc unset", func(t *testing.T) {
		// Arrange
		homeDir := t.TempDir()
		require.NoError(t, (&CliConfig{DockerContext: "fromconfig"}).Save(homeDir))

		// Act
		result := ResolveDockerContext("", nil, homeDir)

		// Assert
		assert.Equal(t, "fromconfig", result)
	})

	t.Run("empty when nothing set", func(t *testing.T) {
		// Act
		result := ResolveDockerContext("", nil, t.TempDir())

		// Assert
		assert.Empty(t, result)
	})
}
