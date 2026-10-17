package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuntimeNames(t *testing.T) {
	// Act
	names := RuntimeNames()

	// Assert
	assert.Equal(t, "docker or microsandbox", names)
}
