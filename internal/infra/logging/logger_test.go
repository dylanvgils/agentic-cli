package logging

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogger_messages covers each message method, plain and f form, on a plain writer.
func TestLogger_messages(t *testing.T) {
	for _, tc := range []struct {
		method string
		write  func(*Logger)
		want   string
	}{
		{method: "Step", write: func(l *Logger) { l.Step("building image") }, want: "=> building image\n"},
		{method: "Stepf", write: func(l *Logger) { l.Stepf("building %s image", "claude") }, want: "=> building claude image\n"},
		{method: "Detail", write: func(l *Logger) { l.Detail("base: java") }, want: "   base: java\n"},
		{method: "Detailf", write: func(l *Logger) { l.Detailf("version: %s", "1.0.0") }, want: "   version: 1.0.0\n"},
		{method: "Info", write: func(l *Logger) { l.Info("starting proxy...") }, want: "agentic: starting proxy...\n"},
		{method: "Infof", write: func(l *Logger) { l.Infof("starting %s...", "proxy") }, want: "agentic: starting proxy...\n"},
		{method: "Error", write: func(l *Logger) { l.Error("no docker") }, want: "agentic: error: no docker\n"},
		{method: "Errorf", write: func(l *Logger) { l.Errorf("no %s", "docker") }, want: "agentic: error: no docker\n"},
		{method: "Warn", write: func(l *Logger) { l.Warn("could not sweep") }, want: "agentic: warning: could not sweep\n"},
		{method: "Warnf", write: func(l *Logger) { l.Warnf("could not %s", "sweep") }, want: "agentic: warning: could not sweep\n"},
		{method: "Prompt", write: func(l *Logger) { l.Prompt("update now? [y/N] ") }, want: "agentic: update now? [y/N] "},
		{method: "Promptf", write: func(l *Logger) { l.Promptf("update %s? [y/N] ", "now") }, want: "agentic: update now? [y/N] "},
	} {
		t.Run(tc.method, func(t *testing.T) {
			// Arrange
			var buf bytes.Buffer
			l := newTestLogger(&buf)

			// Act
			tc.write(l)

			// Assert
			assert.Equal(t, tc.want, buf.String())
		})
	}
}

func TestLogger_Infof(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := newTestLogger(&buf)
	l.color = true

	// Act
	l.Infof("starting %s...", "proxy")

	// Assert - color dims the prefix
	assert.Equal(t, "\x1b[2magentic: \x1b[0mstarting proxy...\n", buf.String())
}

func TestLogger_Blank(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := newTestLogger(&buf)
	l.Infof("starting proxy...")

	// Act
	l.Blank()

	// Assert
	assert.Equal(t, "agentic: starting proxy...\n\n", buf.String())
	assert.False(t, l.pending)
}

func TestLogger_Separate(t *testing.T) {
	t.Run("writes blank line after agentic output", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		l := newTestLogger(&buf)
		l.Infof("starting proxy...")

		// Act
		l.Separate()

		// Assert
		assert.Equal(t, "agentic: starting proxy...\n\n", buf.String())
	})

	t.Run("writes nothing without agentic output", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		l := newTestLogger(&buf)

		// Act
		l.Separate()

		// Assert
		assert.Empty(t, buf.String())
	})

	t.Run("writes only once per batch", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		l := newTestLogger(&buf)
		l.Warnf("could not sweep")
		l.Separate()

		// Act
		l.Separate()

		// Assert
		assert.Equal(t, "agentic: warning: could not sweep\n\n", buf.String())
	})
}

func Test_useColor(t *testing.T) {
	t.Run("non-file writer", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer

		// Act
		got := useColor(&buf)

		// Assert
		assert.False(t, got)
	})

	t.Run("non-terminal file", func(t *testing.T) {
		// Arrange
		f, err := os.CreateTemp(t.TempDir(), "log")
		require.NoError(t, err)
		defer f.Close() //nolint:errcheck

		// Act
		got := useColor(f)

		// Assert
		assert.False(t, got)
	})

	t.Run("NO_COLOR set", func(t *testing.T) {
		// Arrange
		t.Setenv("NO_COLOR", "1")

		// Act
		got := useColor(os.Stderr)

		// Assert
		assert.False(t, got)
	})
}
