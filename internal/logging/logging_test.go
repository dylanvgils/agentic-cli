package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	Stepf("building image")

	// Assert
	assert.Equal(t, "=> building image\n", buf.String())
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
	Detailf("version: 1.0.0")

	// Assert
	assert.Equal(t, "   version: 1.0.0\n", buf.String())
}
