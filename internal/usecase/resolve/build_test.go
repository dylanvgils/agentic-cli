package resolve

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestBases(t *testing.T) {
	t.Run("rc bases are included", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}}}

		// Act
		result := Bases(nil, rc)

		// Assert
		assert.Equal(t, []string{"java"}, result)
	})

	t.Run("flag appends to rc bases", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}}}

		// Act
		result := Bases([]string{"dotnet"}, rc)

		// Assert - sorted by canonical extras order
		assert.Equal(t, []string{"dotnet", "java"}, result)
	})

	t.Run("empty when no sources set", func(t *testing.T) {
		// Act
		result := Bases(nil, &config.AgenticRC{})

		// Assert
		assert.Empty(t, result)
	})
}

func TestVersions(t *testing.T) {
	t.Run("rc versions used as defaults", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{Versions: map[string]string{"java": "17"}}}

		// Act
		result := Versions(nil, rc)

		// Assert
		assert.Equal(t, "17", result["java"])
	})

	t.Run("flag overrides rc version", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{Versions: map[string]string{"java": "17"}}}

		// Act
		result := Versions(map[string]string{"java": "21"}, rc)

		// Assert
		assert.Equal(t, "21", result["java"])
	})

	t.Run("empty when no sources set", func(t *testing.T) {
		// Act
		result := Versions(nil, &config.AgenticRC{})

		// Assert
		assert.Empty(t, result)
	})
}

func TestAptPackages(t *testing.T) {
	t.Run("rc packages are included", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{AptPackages: []string{"make"}}}

		// Act
		result := AptPackages(nil, rc)

		// Assert
		assert.Equal(t, []string{"make"}, result)
	})

	t.Run("flag appends to config packages", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{AptPackages: []string{"make"}}}

		// Act
		result := AptPackages([]string{"gcc"}, rc)

		// Assert
		assert.Equal(t, []string{"make", "gcc"}, result)
	})

	t.Run("empty when no sources set", func(t *testing.T) {
		// Act
		result := AptPackages(nil, &config.AgenticRC{})

		// Assert
		assert.Empty(t, result)
	})
}

func TestBuildOptions(t *testing.T) {
	t.Run("merges bases from flag and rc", func(t *testing.T) {
		// Arrange
		in := BuildInput{Bases: []string{"dotnet", "java"}}

		// Act
		opts := BuildOptions(in, &config.AgenticRC{})

		// Assert
		assert.Equal(t, []string{"dotnet", "java"}, opts.BaseOverride)
	})

	t.Run("verify apt reflects whether any apt packages are set", func(t *testing.T) {
		// Arrange
		in := BuildInput{AptPackages: []string{"make"}}

		// Act
		opts := BuildOptions(in, &config.AgenticRC{})

		// Assert
		assert.True(t, opts.VerifyApt)
	})

	t.Run("custom installs always come from rc", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{CustomInstalls: []config.RCCustomInstall{{Name: "foo"}}}}

		// Act
		opts := BuildOptions(BuildInput{}, rc)

		// Assert
		assert.Equal(t, rc.Build.CustomInstalls, opts.CustomInstalls)
	})
}
