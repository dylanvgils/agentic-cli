package logging

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger_Step(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := newTestLogger(&buf)

	// Act
	l.Step("building image")

	// Assert
	assert.Equal(t, "=> building image\n", buf.String())
}

func TestLogger_Stepf(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := newTestLogger(&buf)

	// Act
	l.Stepf("building %s image", "claude")

	// Assert
	assert.Equal(t, "=> building claude image\n", buf.String())
}

func TestLogger_Detail(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := newTestLogger(&buf)

	// Act
	l.Detail("base: java")

	// Assert
	assert.Equal(t, "   base: java\n", buf.String())
}

func TestLogger_Detailf(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := newTestLogger(&buf)

	// Act
	l.Detailf("version: %s", "1.0.0")

	// Assert
	assert.Equal(t, "   version: 1.0.0\n", buf.String())
}

func TestLogger_Info(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := newTestLogger(&buf)

	// Act
	l.Info("starting proxy...")

	// Assert
	assert.Equal(t, "agentic: starting proxy...\n", buf.String())
}

func TestLogger_Infof(t *testing.T) {
	t.Run("plain writer gets plain prefix", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		l := newTestLogger(&buf)

		// Act
		l.Infof("starting %s...", "proxy")

		// Assert
		assert.Equal(t, "agentic: starting proxy...\n", buf.String())
	})

	t.Run("color dims the prefix", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		l := newTestLogger(&buf)
		l.color = true

		// Act
		l.Infof("starting %s...", "proxy")

		// Assert
		assert.Equal(t, "\x1b[2magentic: \x1b[0mstarting proxy...\n", buf.String())
	})
}

func TestLogger_Error(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := newTestLogger(&buf)

	// Act
	l.Error("no docker")

	// Assert
	assert.Equal(t, "agentic: error: no docker\n", buf.String())
}

func TestLogger_Errorf(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := newTestLogger(&buf)

	// Act
	l.Errorf("no %s", "docker")

	// Assert
	assert.Equal(t, "agentic: error: no docker\n", buf.String())
}

func TestLogger_Warn(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := newTestLogger(&buf)

	// Act
	l.Warn("could not sweep")

	// Assert
	assert.Equal(t, "agentic: warning: could not sweep\n", buf.String())
}

func TestLogger_Warnf(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := newTestLogger(&buf)

	// Act
	l.Warnf("could not %s", "sweep")

	// Assert
	assert.Equal(t, "agentic: warning: could not sweep\n", buf.String())
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

func TestLogger_Prompt(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := newTestLogger(&buf)

	// Act
	l.Prompt("update now? [y/N] ")

	// Assert
	assert.Equal(t, "agentic: update now? [y/N] ", buf.String())
}

func TestLogger_Promptf(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := newTestLogger(&buf)

	// Act
	l.Promptf("update %s? [y/N] ", "now")

	// Assert
	assert.Equal(t, "agentic: update now? [y/N] ", buf.String())
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
