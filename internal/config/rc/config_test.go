package rc

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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
		content := `{"docker_context":"example","log_retention_days":7,"registry":"registry.example.test"}`
		require.NoError(t, os.WriteFile(filepath.Join(dir, "agentic.json"), []byte(content), 0o640))

		// Act
		cfg, err := LoadConfig(dir)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "example", cfg.DockerContext)
		assert.Equal(t, 7, cfg.LogRetentionDays)
		assert.Equal(t, "registry.example.test", cfg.Registry)
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "agentic.json"), []byte(`not-json`), 0o640))

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

func TestCliConfig_LogRetention(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  CliConfig
		want time.Duration
	}{
		{
			name: "uses log_retention_days",
			cfg:  CliConfig{LogRetentionDays: 7},
			want: 7 * 24 * time.Hour,
		},
		{
			name: "falls back to the default",
			cfg:  CliConfig{},
			want: defaultLogRetentionDays * 24 * time.Hour,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			retention := tc.cfg.LogRetention()

			// Assert
			assert.Equal(t, tc.want, retention)
		})
	}
}
