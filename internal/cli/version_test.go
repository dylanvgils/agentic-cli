package cli

import (
	"errors"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/stretchr/testify/assert"
)

var errNotFound = errors.New("not found")

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
	stubDocker(t, &fakeDocker{version: func() (string, error) { return "", errNotFound }})
	stubMicrosandbox(t, &fakeMicrosandbox{version: func() (string, error) { return "", errNotFound }})

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
		assert.Equal(t, "agentic version 1.2.3\n\ncommit       : a1b2c3d", out)
	})

	t.Run("runtime versions in their own section", func(t *testing.T) {
		// Arrange
		buildinfo.Version = "1.2.3"
		buildinfo.Commit = "a1b2c3d"
		buildinfo.BuildDate = ""
		buildinfo.InstallMethod = ""
		stubDocker(t, &fakeDocker{version: func() (string, error) { return "29.9.0", nil }})

		// Act
		out := versionOutput()

		// Assert
		assert.Equal(t, "agentic version 1.2.3\n\ncommit       : a1b2c3d\n\ndocker       : 29.9.0", out)
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
		expected := "commit       : a1b2c3d\nbuilt by     : make\nbuilt date   : 2026-05-18"
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
		expected := "commit       : a1b2c3d\nbuilt date   : 2026-05-18"
		assert.Equal(t, expected, out)
	})
}

func Test_runtimeVersions(t *testing.T) {
	t.Run("docker and msb versions", func(t *testing.T) {
		// Arrange
		stubDocker(t, &fakeDocker{version: func() (string, error) { return "29.9.0", nil }})
		stubMicrosandbox(t, &fakeMicrosandbox{version: func() (string, error) { return "0.2.6", nil }})

		// Act
		out := runtimeVersions()

		// Assert
		assert.Equal(t, "docker       : 29.9.0\nmicrosandbox : 0.2.6", out)
	})

	t.Run("msb not installed omits its line", func(t *testing.T) {
		// Arrange
		stubDocker(t, &fakeDocker{version: func() (string, error) { return "29.9.0", nil }})
		stubMicrosandbox(t, &fakeMicrosandbox{version: func() (string, error) { return "", errNotFound }})

		// Act
		out := runtimeVersions()

		// Assert
		assert.Equal(t, "docker       : 29.9.0", out)
	})
}

func Test_versionOrEmpty(t *testing.T) {
	t.Run("returns the version", func(t *testing.T) {
		// Act
		v := versionOrEmpty(func() (string, error) { return "1.2.3", nil })

		// Assert
		assert.Equal(t, "1.2.3", v)
	})

	t.Run("error returns empty", func(t *testing.T) {
		// Act
		v := versionOrEmpty(func() (string, error) { return "1.2.3", errNotFound })

		// Assert
		assert.Equal(t, "", v)
	})
}
