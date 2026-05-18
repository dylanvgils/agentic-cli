package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildVersion_noMeta(t *testing.T) {
	// Arrange
	version = "1.2.3"
	commit = ""
	buildDate = ""
	installMethod = ""

	// Act
	v := buildVersion()

	// Assert
	assert.Equal(t, "1.2.3", v)
}

func TestBuildVersion_commitOnly(t *testing.T) {
	// Arrange
	version = "1.2.3"
	commit = "a1b2c3d"
	buildDate = ""
	installMethod = ""

	// Act
	v := buildVersion()

	// Assert
	assert.Equal(t, "1.2.3 (a1b2c3d)", v)
}

func TestBuildVersion_commitAndDate(t *testing.T) {
	// Arrange
	version = "1.2.3"
	commit = "a1b2c3d"
	buildDate = "2026-05-18"
	installMethod = ""

	// Act
	v := buildVersion()

	// Assert
	assert.Equal(t, "1.2.3 (a1b2c3d, 2026-05-18)", v)
}

func TestBuildVersion_allFields(t *testing.T) {
	// Arrange
	version = "1.2.3"
	commit = "a1b2c3d"
	buildDate = "2026-05-18"
	installMethod = "make"

	// Act
	v := buildVersion()

	// Assert
	assert.Equal(t, "1.2.3 (a1b2c3d, 2026-05-18, make)", v)
}

func TestBuildVersion_methodOnly(t *testing.T) {
	// Arrange
	version = "1.2.3"
	commit = ""
	buildDate = ""
	installMethod = "script"

	// Act
	v := buildVersion()

	// Assert
	assert.Equal(t, "1.2.3 (script)", v)
}
