package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildVersion_noMethod(t *testing.T) {
	// Arrange
	version = "1.2.3"
	installMethod = ""

	// Act
	v := buildVersion()

	// Assert
	assert.Equal(t, "1.2.3", v)
}

func TestBuildVersion_withMethod(t *testing.T) {
	// Arrange
	version = "1.2.3"
	installMethod = "make"

	// Act
	v := buildVersion()

	// Assert
	assert.Equal(t, "1.2.3 (make)", v)
}

func TestBuildVersion_scriptMethod(t *testing.T) {
	// Arrange
	version = "1.2.3"
	installMethod = "script"

	// Act
	v := buildVersion()

	// Assert
	assert.Equal(t, "1.2.3 (script)", v)
}
