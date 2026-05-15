package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArg_buildsFlag(t *testing.T) {
	// Arrange + Act
	result := arg("volume", "/host:/container")

	// Assert
	assert.Equal(t, "--volume=/host:/container", result)
}

func TestArg_emptyNamePanics(t *testing.T) {
	// Arrange + Act + Assert
	assert.Panics(t, func() { arg("", "value") })
}

func TestArg_dashPrefixPanics(t *testing.T) {
	// Arrange + Act + Assert
	assert.Panics(t, func() { arg("-flag", "value") })
}
