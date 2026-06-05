package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_versionOutput(t *testing.T) {
	t.Run("no metadata", func(t *testing.T) {
		// Arrange
		version = "dev"
		commit = ""
		buildDate = ""
		installMethod = ""

		// Act
		out := versionOutput()

		// Assert
		assert.Equal(t, "agentic version dev", out)
	})

	t.Run("with metadata", func(t *testing.T) {
		// Arrange
		version = "1.2.3"
		commit = "a1b2c3d"
		buildDate = ""
		installMethod = ""

		// Act
		out := versionOutput()

		// Assert
		assert.Equal(t, "agentic version 1.2.3\n\ncommit      : a1b2c3d", out)
	})
}

func Test_versionExtras(t *testing.T) {
	t.Run("no fields", func(t *testing.T) {
		// Arrange
		commit = ""
		buildDate = ""
		installMethod = ""

		// Act
		out := versionExtras()

		// Assert
		assert.Equal(t, "", out)
	})

	t.Run("all fields", func(t *testing.T) {
		// Arrange
		commit = "a1b2c3d"
		installMethod = "make"
		buildDate = "2026-05-18"

		// Act
		out := versionExtras()

		// Assert
		expected := "commit      : a1b2c3d\nbuilt by    : make\nbuilt date  : 2026-05-18"
		assert.Equal(t, expected, out)
	})

	t.Run("partial fields", func(t *testing.T) {
		// Arrange
		commit = "a1b2c3d"
		installMethod = ""
		buildDate = "2026-05-18"

		// Act
		out := versionExtras()

		// Assert
		expected := "commit      : a1b2c3d\nbuilt date  : 2026-05-18"
		assert.Equal(t, expected, out)
	})
}
