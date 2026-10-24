package cli

import (
	"bytes"
	"errors"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/buildinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errNotFound = errors.New("not found")

func Test_writeVersion(t *testing.T) {
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

		var buf bytes.Buffer

		// Act
		err := writeVersion(&buf)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic version dev\n", buf.String())
	})

	t.Run("with metadata", func(t *testing.T) {
		// Arrange
		buildinfo.Version = "1.2.3"
		buildinfo.Commit = "a1b2c3d"
		buildinfo.BuildDate = ""
		buildinfo.InstallMethod = ""

		var buf bytes.Buffer

		// Act
		err := writeVersion(&buf)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic version 1.2.3\n\ncommit:  a1b2c3d\n", buf.String())
	})

	t.Run("runtime versions in their own section", func(t *testing.T) {
		// Arrange
		buildinfo.Version = "1.2.3"
		buildinfo.Commit = "a1b2c3d"
		buildinfo.BuildDate = ""
		buildinfo.InstallMethod = ""
		stubDocker(t, &fakeDocker{version: func() (string, error) { return "29.9.0", nil }})

		var buf bytes.Buffer

		// Act
		err := writeVersion(&buf)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic version 1.2.3\n\ncommit:  a1b2c3d\n\ndocker:  29.9.0\n", buf.String())
	})
}

func Test_versionExtras(t *testing.T) {
	// Arrange
	origCommit := buildinfo.Commit
	origBuildDate := buildinfo.BuildDate
	origInstallMethod := buildinfo.InstallMethod
	t.Cleanup(func() {
		buildinfo.Commit = origCommit
		buildinfo.BuildDate = origBuildDate
		buildinfo.InstallMethod = origInstallMethod
	})
	buildinfo.Commit = "a1b2c3d"
	buildinfo.InstallMethod = "make"
	buildinfo.BuildDate = "2026-05-18"

	// Act
	fields := versionExtras()

	// Assert
	expected := buildDetail{
		commit:    "a1b2c3d",
		builtBy:   "make",
		builtDate: "2026-05-18",
	}
	assert.Equal(t, expected, fields)
}

func Test_runtimeVersions(t *testing.T) {
	t.Run("docker and msb versions", func(t *testing.T) {
		// Arrange
		stubDocker(t, &fakeDocker{version: func() (string, error) { return "29.9.0", nil }})
		stubMicrosandbox(t, &fakeMicrosandbox{version: func() (string, error) { return "0.2.6", nil }})

		// Act
		fields := runtimeVersions()

		// Assert
		expected := runtimeDetail{
			docker:       "29.9.0",
			microsandbox: "0.2.6",
		}
		assert.Equal(t, expected, fields)
	})

	t.Run("msb not installed leaves its version empty", func(t *testing.T) {
		// Arrange
		stubDocker(t, &fakeDocker{version: func() (string, error) { return "29.9.0", nil }})
		stubMicrosandbox(t, &fakeMicrosandbox{version: func() (string, error) { return "", errNotFound }})

		// Act
		fields := runtimeVersions()

		// Assert
		expected := runtimeDetail{docker: "29.9.0"}
		assert.Equal(t, expected, fields)
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
