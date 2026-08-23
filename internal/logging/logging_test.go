package logging

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogger_Step(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := New(&buf)

	// Act
	l.Step("building image")

	// Assert
	assert.Equal(t, "=> building image\n", buf.String())
}

func TestLogger_Stepf(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := New(&buf)

	// Act
	l.Stepf("building %s image", "claude")

	// Assert
	assert.Equal(t, "=> building claude image\n", buf.String())
}

func TestLogger_Detail(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := New(&buf)

	// Act
	l.Detail("base: java")

	// Assert
	assert.Equal(t, "   base: java\n", buf.String())
}

func TestLogger_Detailf(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := New(&buf)

	// Act
	l.Detailf("version: %s", "1.0.0")

	// Assert
	assert.Equal(t, "   version: 1.0.0\n", buf.String())
}

func TestLogger_Writer(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	l := New(&buf)

	// Act
	w := l.Writer()

	// Assert
	assert.Same(t, &buf, w)
}

func TestStep(t *testing.T) {
	// Arrange
	buf := stubLog(t)

	// Act
	Step("building image")

	// Assert
	assert.Equal(t, "=> building image\n", buf.String())
}

func TestStepf(t *testing.T) {
	// Arrange
	buf := stubLog(t)

	// Act
	Stepf("building %s image", "claude")

	// Assert
	assert.Equal(t, "=> building claude image\n", buf.String())
}

func TestDetail(t *testing.T) {
	// Arrange
	buf := stubLog(t)

	// Act
	Detail("base: java")

	// Assert
	assert.Equal(t, "   base: java\n", buf.String())
}

func TestDetailf(t *testing.T) {
	// Arrange
	buf := stubLog(t)

	// Act
	Detailf("version: %s", "1.0.0")

	// Assert
	assert.Equal(t, "   version: 1.0.0\n", buf.String())
}
