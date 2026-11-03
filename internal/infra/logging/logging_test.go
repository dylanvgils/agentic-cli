package logging

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/stretchr/testify/assert"
)

func TestDefault(t *testing.T) {
	t.Run("builds a new logger when the streams changed", func(t *testing.T) {
		// Arrange
		first := iostreamstest.StubErr(t)
		Info("first")
		second := iostreamstest.StubErr(t)

		// Act
		Default().Separate()

		// Assert
		assert.Equal(t, "agentic: first\n", first.String())
		assert.Empty(t, second.String())
	})

	t.Run("keeps the logger while the streams are unchanged", func(t *testing.T) {
		// Arrange
		iostreamstest.StubErr(t)
		want := Default()

		// Act
		got := Default()

		// Assert
		assert.Same(t, want, got)
	})
}

func TestNewLogLogger(t *testing.T) {
	// Arrange
	buf := iostreamstest.StubErr(t)
	l := NewLogLogger()

	// Act
	l.Print("http: TLS handshake error")

	// Assert
	assert.Equal(t, "agentic: warning: http: TLS handshake error\n", buf.String())
}

func TestStep(t *testing.T) {
	// Arrange
	buf := iostreamstest.StubErr(t)

	// Act
	Step("building image")

	// Assert
	assert.Equal(t, "=> building image\n", buf.String())
}

func TestStepf(t *testing.T) {
	// Arrange
	buf := iostreamstest.StubErr(t)

	// Act
	Stepf("building %s image", "claude")

	// Assert
	assert.Equal(t, "=> building claude image\n", buf.String())
}

func TestDetail(t *testing.T) {
	// Arrange
	buf := iostreamstest.StubErr(t)

	// Act
	Detail("base: java")

	// Assert
	assert.Equal(t, "   base: java\n", buf.String())
}

func TestDetailf(t *testing.T) {
	// Arrange
	buf := iostreamstest.StubErr(t)

	// Act
	Detailf("version: %s", "1.0.0")

	// Assert
	assert.Equal(t, "   version: 1.0.0\n", buf.String())
}

func TestInfo(t *testing.T) {
	// Arrange
	buf := iostreamstest.StubErr(t)

	// Act
	Info("starting proxy...")

	// Assert
	assert.Equal(t, "agentic: starting proxy...\n", buf.String())
}

func TestInfof(t *testing.T) {
	// Arrange
	buf := iostreamstest.StubErr(t)

	// Act
	Infof("starting %s...", "proxy")

	// Assert
	assert.Equal(t, "agentic: starting proxy...\n", buf.String())
}

func TestError(t *testing.T) {
	// Arrange
	buf := iostreamstest.StubErr(t)

	// Act
	Error("no docker")

	// Assert
	assert.Equal(t, "agentic: error: no docker\n", buf.String())
}

func TestErrorf(t *testing.T) {
	// Arrange
	buf := iostreamstest.StubErr(t)

	// Act
	Errorf("no %s", "docker")

	// Assert
	assert.Equal(t, "agentic: error: no docker\n", buf.String())
}

func TestWarn(t *testing.T) {
	// Arrange
	buf := iostreamstest.StubErr(t)

	// Act
	Warn("could not sweep")

	// Assert
	assert.Equal(t, "agentic: warning: could not sweep\n", buf.String())
}

func TestWarnf(t *testing.T) {
	// Arrange
	buf := iostreamstest.StubErr(t)

	// Act
	Warnf("could not %s", "sweep")

	// Assert
	assert.Equal(t, "agentic: warning: could not sweep\n", buf.String())
}

func TestBlank(t *testing.T) {
	// Arrange
	buf := iostreamstest.StubErr(t)

	// Act
	Blank()

	// Assert
	assert.Equal(t, "\n", buf.String())
}

func TestSeparate(t *testing.T) {
	// Arrange
	buf := iostreamstest.StubErr(t)
	Infof("starting proxy...")

	// Act
	Separate()

	// Assert
	assert.Equal(t, "agentic: starting proxy...\n\n", buf.String())
}

func TestPrompt(t *testing.T) {
	// Arrange
	buf := iostreamstest.StubErr(t)

	// Act
	Prompt("update now? [y/N] ")

	// Assert
	assert.Equal(t, "agentic: update now? [y/N] ", buf.String())
}

func TestPromptf(t *testing.T) {
	// Arrange
	buf := iostreamstest.StubErr(t)

	// Act
	Promptf("update %s? [y/N] ", "now")

	// Assert
	assert.Equal(t, "agentic: update now? [y/N] ", buf.String())
}
