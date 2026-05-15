package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArg_buildsFlagWithValue(t *testing.T) {
	// Act
	result := arg("volume", "/host:/container")

	// Assert
	assert.Equal(t, "--volume=/host:/container", result)
}

func TestArg_buildsFlagWithoutValue(t *testing.T) {
	// Act
	result := arg("force")

	// Assert
	assert.Equal(t, "--force", result)
}

func TestArg_emptyNamePanics(t *testing.T) {
	// Act + Assert
	assert.Panics(t, func() { arg("") })
}

func TestArg_dashPrefixPanics(t *testing.T) {
	// Act + Assert
	assert.Panics(t, func() { arg("-flag") })
}

func TestArg_multipleValuesPanics(t *testing.T) {
	// Act + Assert
	assert.Panics(t, func() { arg("filter", "a", "b") })
}
