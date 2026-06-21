package cli

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/stretchr/testify/assert"
)

func Test_versionOutput(t *testing.T) {
	origVersion := buildinfo.Version
	origCommit := buildinfo.Commit
	origBuildDate := buildinfo.BuildDate
	origInstallMethod := buildinfo.InstallMethod
	t.Cleanup(func() {
		buildinfo.Version = origVersion
		buildinfo.Commit = origCommit
		buildinfo.BuildDate = origBuildDate
		buildinfo.InstallMethod = origInstallMethod
	})

	t.Run("no metadata", func(t *testing.T) {
		// Arrange
		buildinfo.Version = "dev"
		buildinfo.Commit = ""
		buildinfo.BuildDate = ""
		buildinfo.InstallMethod = ""

		// Act
		out := versionOutput()

		// Assert
		assert.Equal(t, "agentic version dev", out)
	})

	t.Run("with metadata", func(t *testing.T) {
		// Arrange
		buildinfo.Version = "1.2.3"
		buildinfo.Commit = "a1b2c3d"
		buildinfo.BuildDate = ""
		buildinfo.InstallMethod = ""

		// Act
		out := versionOutput()

		// Assert
		assert.Equal(t, "agentic version 1.2.3\n\ncommit      : a1b2c3d", out)
	})
}

func Test_versionExtras(t *testing.T) {
	origCommit := buildinfo.Commit
	origBuildDate := buildinfo.BuildDate
	origInstallMethod := buildinfo.InstallMethod
	t.Cleanup(func() {
		buildinfo.Commit = origCommit
		buildinfo.BuildDate = origBuildDate
		buildinfo.InstallMethod = origInstallMethod
	})

	t.Run("no fields", func(t *testing.T) {
		// Arrange
		buildinfo.Commit = ""
		buildinfo.BuildDate = ""
		buildinfo.InstallMethod = ""

		// Act
		out := versionExtras()

		// Assert
		assert.Equal(t, "", out)
	})

	t.Run("all fields", func(t *testing.T) {
		// Arrange
		buildinfo.Commit = "a1b2c3d"
		buildinfo.InstallMethod = "make"
		buildinfo.BuildDate = "2026-05-18"

		// Act
		out := versionExtras()

		// Assert
		expected := "commit      : a1b2c3d\nbuilt by    : make\nbuilt date  : 2026-05-18"
		assert.Equal(t, expected, out)
	})

	t.Run("partial fields", func(t *testing.T) {
		// Arrange
		buildinfo.Commit = "a1b2c3d"
		buildinfo.InstallMethod = ""
		buildinfo.BuildDate = "2026-05-18"

		// Act
		out := versionExtras()

		// Assert
		expected := "commit      : a1b2c3d\nbuilt date  : 2026-05-18"
		assert.Equal(t, expected, out)
	})
}
