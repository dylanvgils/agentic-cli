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
