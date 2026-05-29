package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildVersion(t *testing.T) {
	t.Run("no meta", func(t *testing.T) {
		// Arrange
		version = "1.2.3"
		commit = ""
		buildDate = ""
		installMethod = ""

		// Act
		v := buildVersion()

		// Assert
		assert.Equal(t, "1.2.3", v)
	})

	t.Run("commit only", func(t *testing.T) {
		// Arrange
		version = "1.2.3"
		commit = "a1b2c3d"
		buildDate = ""
		installMethod = ""

		// Act
		v := buildVersion()

		// Assert
		assert.Equal(t, "1.2.3 (a1b2c3d)", v)
	})

	t.Run("commit and date", func(t *testing.T) {
		// Arrange
		version = "1.2.3"
		commit = "a1b2c3d"
		buildDate = "2026-05-18"
		installMethod = ""

		// Act
		v := buildVersion()

		// Assert
		assert.Equal(t, "1.2.3 (a1b2c3d, 2026-05-18)", v)
	})

	t.Run("all fields", func(t *testing.T) {
		// Arrange
		version = "1.2.3"
		commit = "a1b2c3d"
		buildDate = "2026-05-18"
		installMethod = "make"

		// Act
		v := buildVersion()

		// Assert
		assert.Equal(t, "1.2.3 (a1b2c3d, 2026-05-18, make)", v)
	})

	t.Run("method only", func(t *testing.T) {
		// Arrange
		version = "1.2.3"
		commit = ""
		buildDate = ""
		installMethod = "script"

		// Act
		v := buildVersion()

		// Assert
		assert.Equal(t, "1.2.3 (script)", v)
	})
}
