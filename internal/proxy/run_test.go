package proxy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigFromEnv(t *testing.T) {
	t.Run("parses comma-separated hosts and trims blanks", func(t *testing.T) {
		// Arrange
		t.Setenv(EnvAllow, " api.anthropic.com , ,.github.com ")
		t.Setenv(EnvLog, "/var/log/proxy.jsonl")
		t.Setenv(EnvAddr, ":9999")

		// Act
		cfg := ConfigFromEnv()

		// Assert
		assert.Equal(t, []string{"api.anthropic.com", ".github.com"}, cfg.AllowedHosts)
		assert.Equal(t, "/var/log/proxy.jsonl", cfg.LogPath)
		assert.Equal(t, ":9999", cfg.Addr)
	})

	t.Run("empty allow yields no hosts", func(t *testing.T) {
		// Arrange
		t.Setenv(EnvAllow, "")

		// Act
		cfg := ConfigFromEnv()

		// Assert
		assert.Empty(t, cfg.AllowedHosts)
	})

	t.Run("parses a negative TZ offset", func(t *testing.T) {
		// Arrange
		t.Setenv(EnvTZOffset, "-18000")

		// Act
		cfg := ConfigFromEnv()

		// Assert
		assert.Equal(t, -18000, cfg.TZOffsetSeconds)
	})

	t.Run("missing or invalid TZ offset defaults to zero", func(t *testing.T) {
		// Arrange
		t.Setenv(EnvTZOffset, "not-a-number")

		// Act
		cfg := ConfigFromEnv()

		// Assert
		assert.Zero(t, cfg.TZOffsetSeconds)
	})

	t.Run("monitor true enables monitor mode", func(t *testing.T) {
		// Arrange
		t.Setenv(EnvMonitor, "true")

		// Act
		cfg := ConfigFromEnv()

		// Assert
		assert.True(t, cfg.Monitor)
	})

	t.Run("missing or invalid monitor defaults to false", func(t *testing.T) {
		// Arrange
		t.Setenv(EnvMonitor, "not-a-bool")

		// Act
		cfg := ConfigFromEnv()

		// Assert
		assert.False(t, cfg.Monitor)
	})
}

func TestOpenLog(t *testing.T) {
	t.Run("empty path returns no file", func(t *testing.T) {
		// Act
		f, closeFn, err := openLog("")
		defer closeFn()

		// Assert
		require.NoError(t, err)
		assert.Nil(t, f)
	})

	t.Run("file path opens a writable file", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "proxy.jsonl")

		// Act
		f, closeFn, err := openLog(path)
		defer closeFn()

		// Assert
		require.NoError(t, err)
		_, err = f.Write([]byte("hello\n"))
		require.NoError(t, err)

		fileContents, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "hello\n", string(fileContents))
	})

	t.Run("invalid path returns error", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "missing-dir", "proxy.jsonl")

		// Act
		_, _, err := openLog(path)

		// Assert
		assert.Error(t, err)
	})
}
