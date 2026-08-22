package housekeeping

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPruneJSONLLogs(t *testing.T) {
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
		PruneJSONLLogs(dir, 24*time.Hour)

		// Assert
		assert.NoFileExists(t, old)
		assert.FileExists(t, recent)
	})

	t.Run("ignores non-jsonl files regardless of age", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		other := writeFile(t, dir, "old.txt", 48*time.Hour)

		// Act
		PruneJSONLLogs(dir, 24*time.Hour)

		// Assert
		assert.FileExists(t, other)
	})

	t.Run("maxAge of zero removes every log file", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		old := writeFile(t, dir, "old.jsonl", 48*time.Hour)
		recent := writeFile(t, dir, "recent.jsonl", time.Minute)

		// Act
		PruneJSONLLogs(dir, 0)

		// Assert
		assert.NoFileExists(t, old)
		assert.NoFileExists(t, recent)
	})

	t.Run("missing directory is a no-op", func(t *testing.T) {
		// Arrange
		dir := filepath.Join(t.TempDir(), "absent")

		// Act + Assert
		assert.NotPanics(t, func() { PruneJSONLLogs(dir, 24*time.Hour) })
	})
}

func TestFollowJSONL(t *testing.T) {
	const pollInterval = 10 * time.Millisecond

	readLine := func(t *testing.T, lines <-chan []byte) string {
		t.Helper()
		select {
		case line, ok := <-lines:
			require.True(t, ok, "channel closed before a line arrived")
			return string(line)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for a line")
			return ""
		}
	}

	t.Run("replays pre-existing content from the start", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "run.jsonl")
		require.NoError(t, os.WriteFile(path, []byte(`{"op":"open"}`+"\n"), 0o644))

		// Act
		lines, stop := FollowJSONL(path, pollInterval)
		t.Cleanup(stop)

		// Assert
		assert.Equal(t, `{"op":"open"}`, readLine(t, lines))
	})

	t.Run("delivers lines appended after following starts, in order", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "run.jsonl")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		require.NoError(t, err)
		t.Cleanup(func() { _ = f.Close() })

		// Act
		lines, stop := FollowJSONL(path, pollInterval)
		t.Cleanup(stop)
		_, err = f.WriteString(`{"op":"write","path":"a"}` + "\n")
		require.NoError(t, err)
		_, err = f.WriteString(`{"op":"write","path":"b"}` + "\n")
		require.NoError(t, err)

		// Assert
		assert.Equal(t, `{"op":"write","path":"a"}`, readLine(t, lines))
		assert.Equal(t, `{"op":"write","path":"b"}`, readLine(t, lines))
	})

	t.Run("picks up lines once a not-yet-existing path is created", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "run.jsonl")
		lines, stop := FollowJSONL(path, pollInterval)
		t.Cleanup(stop)

		// Act
		time.Sleep(3 * pollInterval)
		require.NoError(t, os.WriteFile(path, []byte(`{"detail":"late"}`+"\n"), 0o644))

		// Assert
		assert.Equal(t, `{"detail":"late"}`, readLine(t, lines))
	})

	t.Run("stop closes the channel and halts further reads", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "run.jsonl")
		require.NoError(t, os.WriteFile(path, nil, 0o644))
		lines, stop := FollowJSONL(path, pollInterval)

		// Act
		stop()

		// Assert
		select {
		case _, ok := <-lines:
			assert.False(t, ok)
		case <-time.After(2 * time.Second):
			t.Fatal("channel was not closed after stop")
		}
	})
}
