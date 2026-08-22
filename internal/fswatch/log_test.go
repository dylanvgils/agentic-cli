package fswatch

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerLog(t *testing.T) {
	t.Run("writes a JSON line with the given fields", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		logger := NewLogger(&buf)
		fixed := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
		logger.now = func() time.Time { return fixed }

		// Act
		logger.Log(OpWrite, "/workspace/main.go", false)

		// Assert
		var entry Entry
		require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
		assert.Equal(t, OpWrite, entry.Op)
		assert.Equal(t, "/workspace/main.go", entry.Path)
		assert.False(t, entry.IsDir)
		assert.True(t, entry.Time.Equal(fixed))
		assert.Empty(t, entry.Detail)
	})

	t.Run("marks directory entries", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		logger := NewLogger(&buf)

		// Act
		logger.Log(OpCreate, "/workspace/sub", true)

		// Assert
		var entry Entry
		require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
		assert.True(t, entry.IsDir)
	})
}

func TestLoggerLogDetail(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	logger := NewLogger(&buf)

	// Act
	logger.LogDetail("inotify queue overflow: some events were dropped")

	// Assert
	var entry Entry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "inotify queue overflow: some events were dropped", entry.Detail)
	assert.Empty(t, entry.Op)
	assert.Empty(t, entry.Path)
}
