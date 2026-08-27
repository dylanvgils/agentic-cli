package migrations

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoveToolAndProxyDirs(t *testing.T) {
	writeFile := func(t *testing.T, path, content string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o640))
	}

	t.Run("moves existing tool dirs under tools/", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		writeFile(t, filepath.Join(home, "claude", "data", "x"), "claude-data")
		writeFile(t, filepath.Join(home, "claude", ".claude.json"), "{}")
		writeFile(t, filepath.Join(home, "copilot", "y"), "copilot-data")
		writeFile(t, filepath.Join(home, "opencode", "data", "z"), "opencode-data")

		// Act
		err := MoveToolAndProxyDirs(home)

		// Assert
		require.NoError(t, err)
		assertFileContent(t, filepath.Join(home, tools.ToolsDirName, "claude", "data", "x"), "claude-data")
		assertFileContent(t, filepath.Join(home, tools.ToolsDirName, "claude", ".claude.json"), "{}")
		assertFileContent(t, filepath.Join(home, tools.ToolsDirName, "copilot", "y"), "copilot-data")
		assertFileContent(t, filepath.Join(home, tools.ToolsDirName, "opencode", "data", "z"), "opencode-data")
		assert.NoDirExists(t, filepath.Join(home, "claude"))
		assert.NoDirExists(t, filepath.Join(home, "copilot"))
		assert.NoDirExists(t, filepath.Join(home, "opencode"))
	})

	t.Run("moves proxy logs into logs/ with the proxy_ prefix", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		writeFile(t, filepath.Join(home, "proxy", "aaa111.jsonl"), "log-1")
		writeFile(t, filepath.Join(home, "proxy", "bbb222.jsonl"), "log-2")

		// Act
		err := MoveToolAndProxyDirs(home)

		// Assert
		require.NoError(t, err)
		assertFileContent(t, filepath.Join(home, config.LogsDirName, proxy.LogFilePrefix+"aaa111.jsonl"), "log-1")
		assertFileContent(t, filepath.Join(home, config.LogsDirName, proxy.LogFilePrefix+"bbb222.jsonl"), "log-2")
		assert.NoDirExists(t, filepath.Join(home, "proxy"))
	})

	t.Run("no-op when nothing to move", func(t *testing.T) {
		// Arrange
		home := t.TempDir()

		// Act
		err := MoveToolAndProxyDirs(home)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, filepath.Join(home, tools.ToolsDirName))
		assert.DirExists(t, filepath.Join(home, config.LogsDirName))
	})

	t.Run("idempotent on retry after partial failure", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		writeFile(t, filepath.Join(home, tools.ToolsDirName, "claude", "data", "x"), "claude-data")
		writeFile(t, filepath.Join(home, "copilot", "y"), "copilot-data")
		writeFile(t, filepath.Join(home, "proxy", "aaa111.jsonl"), "log-1")

		// Act
		err := MoveToolAndProxyDirs(home)

		// Assert
		require.NoError(t, err)
		assertFileContent(t, filepath.Join(home, tools.ToolsDirName, "claude", "data", "x"), "claude-data")
		assertFileContent(t, filepath.Join(home, tools.ToolsDirName, "copilot", "y"), "copilot-data")
		assert.NoDirExists(t, filepath.Join(home, "copilot"))
		assertFileContent(t, filepath.Join(home, config.LogsDirName, proxy.LogFilePrefix+"aaa111.jsonl"), "log-1")
		assert.NoDirExists(t, filepath.Join(home, "proxy"))

		// Act again
		err = MoveToolAndProxyDirs(home)

		// Assert again
		require.NoError(t, err)
		assertFileContent(t, filepath.Join(home, tools.ToolsDirName, "claude", "data", "x"), "claude-data")
		assertFileContent(t, filepath.Join(home, tools.ToolsDirName, "copilot", "y"), "copilot-data")
		assertFileContent(t, filepath.Join(home, config.LogsDirName, proxy.LogFilePrefix+"aaa111.jsonl"), "log-1")
	})

	t.Run("leaves marketplaces, agentic.json, and .migrations.json untouched", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		writeFile(t, filepath.Join(home, "marketplaces", "clone", "file"), "marketplace-data")
		writeFile(t, filepath.Join(home, "agentic.json"), "{}")
		writeFile(t, filepath.Join(home, ".migrations.json"), `{"version":1}`)

		// Act
		err := MoveToolAndProxyDirs(home)

		// Assert
		require.NoError(t, err)
		assertFileContent(t, filepath.Join(home, "marketplaces", "clone", "file"), "marketplace-data")
		assertFileContent(t, filepath.Join(home, "agentic.json"), "{}")
		assertFileContent(t, filepath.Join(home, ".migrations.json"), `{"version":1}`)
	})
}
