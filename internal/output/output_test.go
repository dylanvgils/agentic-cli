package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStep(t *testing.T) {
	// Act
	got := captureStdout(t, func() { Step("building image") })

	// Assert
	assert.Equal(t, "=> building image\n", got)
}

func TestStepf(t *testing.T) {
	// Act
	got := captureStdout(t, func() { Stepf("building %s image", "claude") })

	// Assert
	assert.Equal(t, "=> building claude image\n", got)
}

func TestDetail(t *testing.T) {
	// Act
	got := captureStdout(t, func() { Detail("base: java") })

	// Assert
	assert.Equal(t, "   base: java\n", got)
}

func TestDetailf(t *testing.T) {
	// Act
	got := captureStdout(t, func() { Detailf("version: %s", "1.0.0") })

	// Assert
	assert.Equal(t, "   version: 1.0.0\n", got)
}
