package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrint(t *testing.T) {
	// Arrange
	buf := stubOut(t)

	// Act
	Print("FROM scratch")

	// Assert
	assert.Equal(t, "FROM scratch", buf.String())
}

func TestPrintln(t *testing.T) {
	// Arrange
	buf := stubOut(t)

	// Act
	Println("alias agt=agentic")

	// Assert
	assert.Equal(t, "alias agt=agentic\n", buf.String())
}

func TestStep(t *testing.T) {
	// Arrange
	buf := stubErr(t)

	// Act
	Step("building image")

	// Assert
	assert.Equal(t, "=> building image\n", buf.String())
}

func TestStepf(t *testing.T) {
	// Arrange
	buf := stubErr(t)

	// Act
	Stepf("building %s image", "claude")

	// Assert
	assert.Equal(t, "=> building claude image\n", buf.String())
}

func TestDetail(t *testing.T) {
	// Arrange
	buf := stubErr(t)

	// Act
	Detail("base: java")

	// Assert
	assert.Equal(t, "   base: java\n", buf.String())
}

func TestDetailf(t *testing.T) {
	// Arrange
	buf := stubErr(t)

	// Act
	Detailf("version: %s", "1.0.0")

	// Assert
	assert.Equal(t, "   version: 1.0.0\n", buf.String())
}

func TestInfof(t *testing.T) {
	// Arrange
	buf := stubErr(t)

	// Act
	Infof("starting %s...", "proxy")

	// Assert
	assert.Equal(t, "agentic: starting proxy...\n", buf.String())
}

func TestErrorf(t *testing.T) {
	// Arrange
	buf := stubErr(t)

	// Act
	Errorf("no %s", "docker")

	// Assert
	assert.Equal(t, "agentic: error: no docker\n", buf.String())
}

func TestWarnf(t *testing.T) {
	// Arrange
	buf := stubErr(t)

	// Act
	Warnf("could not %s", "sweep")

	// Assert
	assert.Equal(t, "agentic: warning: could not sweep\n", buf.String())
}

func TestSeparate(t *testing.T) {
	// Arrange
	buf := stubErr(t)
	Infof("starting proxy...")

	// Act
	Separate()

	// Assert
	assert.Equal(t, "agentic: starting proxy...\n\n", buf.String())
}

func TestPromptf(t *testing.T) {
	// Arrange
	buf := stubErr(t)

	// Act
	Promptf("update %s? [y/N] ", "now")

	// Assert
	assert.Equal(t, "agentic: update now? [y/N] ", buf.String())
}
