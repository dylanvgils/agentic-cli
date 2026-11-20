package logging

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDir(t *testing.T) {
	// Arrange
	start := time.Date(2026, 11, 20, 15, 30, 45, 0, time.Local)

	// Act
	dir := Dir("/example.test/home", "build", start)

	// Assert
	assert.Equal(t, filepath.Join("/example.test/home", DirName, "20261120-153045-build"), dir)
}

func TestPrune(t *testing.T) {
	old := time.Now().Add(-48 * time.Hour)
	recent := time.Now().Add(-time.Hour)

	for _, tc := range []struct {
		name    string
		create  func(t *testing.T, logs string) string
		maxAge  time.Duration
		removed bool
	}{
		{
			name:    "removes an old run dir with its logs",
			create:  runLogDirAt(old),
			maxAge:  24 * time.Hour,
			removed: true,
		},
		{
			name: "removes an old log dir of another command",
			create: func(t *testing.T, logs string) string {
				dir := Dir(filepath.Dir(logs), "build", old)
				require.NoError(t, os.Mkdir(dir, 0o750))
				return dir
			},
			maxAge:  24 * time.Hour,
			removed: true,
		},
		{
			name:    "keeps a recent run dir",
			create:  runLogDirAt(recent),
			maxAge:  24 * time.Hour,
			removed: false,
		},
		{
			name:    "removes an old loose log file",
			create:  looseLogAt(old),
			maxAge:  24 * time.Hour,
			removed: true,
		},
		{
			name:    "keeps a recent loose log file",
			create:  looseLogAt(recent),
			maxAge:  24 * time.Hour,
			removed: false,
		},
		{
			name: "keeps an undated dir",
			create: func(t *testing.T, logs string) string {
				dir := filepath.Join(logs, "other")
				require.NoError(t, os.Mkdir(dir, 0o750))
				require.NoError(t, os.Chtimes(dir, old, old))
				return dir
			},
			maxAge:  24 * time.Hour,
			removed: false,
		},
		{
			name:    "maxAge of zero removes a recent run dir",
			create:  runLogDirAt(recent),
			maxAge:  0,
			removed: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			home := t.TempDir()
			logs := filepath.Join(home, DirName)
			require.NoError(t, os.Mkdir(logs, 0o750))
			path := tc.create(t, logs)

			// Act
			Prune(home, tc.maxAge)

			// Assert
			_, err := os.Stat(path)
			assert.Equal(t, tc.removed, os.IsNotExist(err), "removed")
		})
	}

	t.Run("missing logs dir is a no-op", func(t *testing.T) {
		// Arrange
		home := filepath.Join(t.TempDir(), "absent")

		// Act + Assert
		assert.NotPanics(t, func() { Prune(home, 24*time.Hour) })
	})
}

// runLogDirAt returns a fixture creating a run's log dir started at start, holding a log file.
func runLogDirAt(start time.Time) func(t *testing.T, logs string) string {
	return func(t *testing.T, logs string) string {
		dir := Dir(filepath.Dir(logs), "0123456789ab", start)
		require.NoError(t, os.Mkdir(dir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "proxy.jsonl"), []byte("{}\n"), 0o600))
		return dir
	}
}

// looseLogAt returns a fixture creating a flat log file last modified at mtime.
func looseLogAt(mtime time.Time) func(t *testing.T, logs string) string {
	return func(t *testing.T, logs string) string {
		path := filepath.Join(logs, "proxy_0123456789ab.jsonl")
		require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0o600))
		require.NoError(t, os.Chtimes(path, mtime, mtime))
		return path
	}
}
