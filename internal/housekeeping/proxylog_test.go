package housekeeping

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPruneProxyLogs(t *testing.T) {
	writeFile := func(t *testing.T, dir, name string, age time.Duration) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0o644))
		require.NoError(t, os.Chtimes(path, time.Now().Add(-age), time.Now().Add(-age)))
		return path
	}

	t.Run("removes files older than maxAge and keeps recent ones", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		old := writeFile(t, dir, "old.jsonl", 48*time.Hour)
		recent := writeFile(t, dir, "recent.jsonl", time.Hour)

		// Act
		PruneProxyLogs(dir, 24*time.Hour)

		// Assert
		assert.NoFileExists(t, old)
		assert.FileExists(t, recent)
	})

	t.Run("ignores non-jsonl files regardless of age", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		other := writeFile(t, dir, "old.txt", 48*time.Hour)

		// Act
		PruneProxyLogs(dir, 24*time.Hour)

		// Assert
		assert.FileExists(t, other)
	})

	t.Run("maxAge of zero removes every log file", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		old := writeFile(t, dir, "old.jsonl", 48*time.Hour)
		recent := writeFile(t, dir, "recent.jsonl", time.Minute)

		// Act
		PruneProxyLogs(dir, 0)

		// Assert
		assert.NoFileExists(t, old)
		assert.NoFileExists(t, recent)
	})

	t.Run("missing directory is a no-op", func(t *testing.T) {
		// Arrange
		dir := filepath.Join(t.TempDir(), "absent")

		// Act + Assert
		assert.NotPanics(t, func() { PruneProxyLogs(dir, 24*time.Hour) })
	})
}
