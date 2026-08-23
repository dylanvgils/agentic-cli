package resolve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNamespace(t *testing.T) {
	t.Run("flag takes priority over rc", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Namespace: "fromrc"}

		// Act
		result := Namespace("fromflag", rc)

		// Assert
		assert.Equal(t, "fromflag", result)
	})

	t.Run("rc value used when flag absent", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Namespace: "fromrc"}

		// Act
		result := Namespace("", rc)

		// Assert
		assert.Equal(t, "fromrc", result)
	})

	t.Run("falls back to default when nothing set", func(t *testing.T) {
		// Act
		result := Namespace("", nil)

		// Assert
		assert.Equal(t, config.DefaultNamespace, result)
	})
}

func TestRegistry(t *testing.T) {
	t.Run("flag takes priority over config", func(t *testing.T) {
		// Arrange
		homeDir := t.TempDir()
		cfg := &config.CliConfig{Registry: "config.example.com"}
		require.NoError(t, cfg.Save(homeDir))

		// Act
		result := Registry("flag.example.com", homeDir)

		// Assert
		assert.Equal(t, "flag.example.com", result)
	})

	t.Run("falls back to agentic.json when flag not set", func(t *testing.T) {
		// Arrange
		homeDir := t.TempDir()
		cfg := &config.CliConfig{Registry: "config.example.com"}
		require.NoError(t, cfg.Save(homeDir))

		// Act
		result := Registry("", homeDir)

		// Assert
		assert.Equal(t, "config.example.com", result)
	})

	t.Run("empty when neither set", func(t *testing.T) {
		// Act
		result := Registry("", t.TempDir())

		// Assert
		assert.Empty(t, result)
	})

	t.Run("empty when homeDir has no config file", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "agentic.json"), []byte("invalid json"), 0o644))

		// Act
		result := Registry("", dir)

		// Assert
		assert.Empty(t, result)
	})
}

func TestDockerContext(t *testing.T) {
	t.Run("flag wins over everything", func(t *testing.T) {
		// Arrange
		homeDir := t.TempDir()
		require.NoError(t, (&config.CliConfig{DockerContext: "fromconfig"}).Save(homeDir))
		rc := &config.AgenticRC{DockerContext: "fromrc"}

		// Act
		result := DockerContext("fromflag", rc, homeDir)

		// Assert
		assert.Equal(t, "fromflag", result)
	})

	t.Run("rc value wins over agentic.json", func(t *testing.T) {
		// Arrange
		homeDir := t.TempDir()
		require.NoError(t, (&config.CliConfig{DockerContext: "fromconfig"}).Save(homeDir))
		rc := &config.AgenticRC{DockerContext: "fromrc"}

		// Act
		result := DockerContext("", rc, homeDir)

		// Assert
		assert.Equal(t, "fromrc", result)
	})

	t.Run("agentic.json wins when rc unset", func(t *testing.T) {
		// Arrange
		homeDir := t.TempDir()
		require.NoError(t, (&config.CliConfig{DockerContext: "fromconfig"}).Save(homeDir))

		// Act
		result := DockerContext("", nil, homeDir)

		// Assert
		assert.Equal(t, "fromconfig", result)
	})

	t.Run("empty when nothing set", func(t *testing.T) {
		// Act
		result := DockerContext("", nil, t.TempDir())

		// Assert
		assert.Empty(t, result)
	})
}
