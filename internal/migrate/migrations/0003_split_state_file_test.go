package migrations

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitStateFile(t *testing.T) {
	t.Run("moves state keys to state.json and keeps the settings", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		legacy := `{"registry":"registry.example.test","trusted_dirs":["/example.test/projects"],"last_update_check":"2026-01-01T00:00:00Z","last_tool_version_check":{"claude":"2026-01-01T00:00:00Z"},"approved_settings":{"/example.test/.agenticrc.toml":{"root":"abc"}}}`
		require.NoError(t, os.WriteFile(filepath.Join(home, "agentic.json"), []byte(legacy), 0o640))

		// Act
		err := SplitStateFile(home)

		// Assert
		require.NoError(t, err)
		assertJSONFile(t, filepath.Join(home, "agentic.json"), `{"registry":"registry.example.test"}`)
		assertJSONFile(t, filepath.Join(home, "state.json"), `{"trusted_dirs":["/example.test/projects"],"last_update_check":"2026-01-01T00:00:00Z","last_tool_version_check":{"claude":"2026-01-01T00:00:00Z"},"approved_settings":{"/example.test/.agenticrc.toml":{"root":"abc"}}}`)
	})

	t.Run("no-op without state keys", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		settings := `{"registry": "registry.example.test"}`
		require.NoError(t, os.WriteFile(filepath.Join(home, "agentic.json"), []byte(settings), 0o640))

		// Act
		err := SplitStateFile(home)

		// Assert
		require.NoError(t, err)
		assertFileContent(t, filepath.Join(home, "agentic.json"), settings)
		assert.NoFileExists(t, filepath.Join(home, "state.json"))
	})

	t.Run("no-op without agentic.json", func(t *testing.T) {
		// Arrange
		home := t.TempDir()

		// Act
		err := SplitStateFile(home)

		// Assert
		require.NoError(t, err)
		assert.NoFileExists(t, filepath.Join(home, "agentic.json"))
		assert.NoFileExists(t, filepath.Join(home, "state.json"))
	})

	t.Run("retry after a partial failure keeps the state.json already written", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(home, "agentic.json"), []byte(`{"trusted_dirs":["/example.test/old"]}`), 0o640))
		require.NoError(t, os.WriteFile(filepath.Join(home, "state.json"), []byte(`{"trusted_dirs":["/example.test/new"]}`), 0o640))

		// Act
		err := SplitStateFile(home)

		// Assert
		require.NoError(t, err)
		assertJSONFile(t, filepath.Join(home, "agentic.json"), `{}`)
		assertJSONFile(t, filepath.Join(home, "state.json"), `{"trusted_dirs":["/example.test/new"]}`)
	})

	t.Run("malformed agentic.json is an error", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(home, "agentic.json"), []byte(`not-json`), 0o640))

		// Act
		err := SplitStateFile(home)

		// Assert
		require.Error(t, err)
	})
}
