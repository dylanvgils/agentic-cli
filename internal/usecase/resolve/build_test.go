package resolve

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/stretchr/testify/assert"
)

func TestBases(t *testing.T) {
	rc := &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}}}

	for _, tc := range []struct {
		name  string
		flags []string
		rc    *config.AgenticRC
		want  []string
	}{
		{name: "rc bases are included", rc: rc, want: []string{"java"}},
		{name: "flag appends to rc bases in canonical order", flags: []string{"dotnet"}, rc: rc, want: []string{"dotnet", "java"}},
		{name: "empty when no sources set", rc: &config.AgenticRC{}, want: []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := Bases(tc.flags, tc.rc)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestBasesExact(t *testing.T) {
	t.Run("returns sorted flag values only", func(t *testing.T) {
		// Act
		result := BasesExact([]string{"java", "dotnet"})

		// Assert - sorted by canonical extras order
		assert.Equal(t, []string{"dotnet", "java"}, result)
	})

	t.Run("empty flag list returns empty", func(t *testing.T) {
		// Act
		result := BasesExact(nil)

		// Assert
		assert.Empty(t, result)
	})
}

func TestVersions(t *testing.T) {
	rc := &config.AgenticRC{Build: config.RCBuild{Versions: map[string]string{"java": "17"}}}

	for _, tc := range []struct {
		name  string
		flags map[string]string
		rc    *config.AgenticRC
		want  map[string]string
	}{
		{name: "rc versions used as defaults", rc: rc, want: map[string]string{"java": "17"}},
		{name: "flag overrides rc version", flags: map[string]string{"java": "21"}, rc: rc, want: map[string]string{"java": "21"}},
		{name: "empty flag value keeps rc version", flags: map[string]string{"java": ""}, rc: rc, want: map[string]string{"java": "17"}},
		{name: "empty when no sources set", rc: &config.AgenticRC{}, want: map[string]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := Versions(tc.flags, tc.rc)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestAptPackages(t *testing.T) {
	rc := &config.AgenticRC{Build: config.RCBuild{AptPackages: []string{"make"}}}

	for _, tc := range []struct {
		name  string
		flags []string
		rc    *config.AgenticRC
		want  []string
	}{
		{name: "rc packages are included", rc: rc, want: []string{"make"}},
		{name: "flag appends to config packages", flags: []string{"gcc"}, rc: rc, want: []string{"make", "gcc"}},
		{name: "empty when no sources set", rc: &config.AgenticRC{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := AptPackages(tc.flags, tc.rc)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestAptPackagesExact(t *testing.T) {
	t.Run("returns deduplicated flag values only", func(t *testing.T) {
		// Act
		result := AptPackagesExact([]string{"make", "gcc", "make"})

		// Assert
		assert.Equal(t, []string{"make", "gcc"}, result)
	})

	t.Run("empty flag list returns empty", func(t *testing.T) {
		// Act
		result := AptPackagesExact(nil)

		// Assert
		assert.Empty(t, result)
	})
}

func TestBuildOptions(t *testing.T) {
	t.Run("merges the flags with rc", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{
			Bases:          []string{"java"},
			AptPackages:    []string{"jq"},
			Versions:       map[string]string{"java": "17"},
			CustomInstalls: []config.RCCustomInstall{{Name: "foo"}},
		}}
		in := BuildInput{
			Bases:               []string{"dotnet"},
			VersionOverrides:    map[string]string{"node": "24"},
			AptPackages:         []string{"make"},
			NoCache:             true,
			Pull:                true,
			Registry:            "registry.example.test",
			SkipInstallChecksum: true,
		}

		// Act
		opts := BuildOptions(in, rc)

		// Assert
		assert.Equal(t, tools.BuildOptions{
			BaseOverride:        []string{"dotnet", "java"},
			Versions:            map[string]string{"java": "17", "node": "24"},
			AptPackages:         []string{"jq", "make"},
			VerifyApt:           true,
			NoCache:             true,
			Pull:                true,
			Registry:            "registry.example.test",
			CustomInstalls:      rc.Build.CustomInstalls,
			SkipInstallChecksum: true,
		}, opts)
	})

	rc := &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}, AptPackages: []string{"make"}}}

	for _, tc := range []struct {
		name       string
		basesExact []string
		aptExact   []string
		wantBases  []string
		wantApt    []string
	}{
		{name: "base-exact overrides rc bases entirely", basesExact: []string{"node"}, wantBases: []string{"node"}, wantApt: []string{"make"}},
		{name: "empty base-exact clears rc bases", basesExact: []string{}, wantApt: []string{"make"}},
		{name: "apt-exact overrides rc apt entirely", aptExact: []string{"gcc"}, wantBases: []string{"java"}, wantApt: []string{"gcc"}},
		{name: "empty apt-exact clears rc apt and disables verify", aptExact: []string{}, wantBases: []string{"java"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			var in BuildInput
			if tc.basesExact != nil {
				in.BasesExact = &tc.basesExact
			}
			if tc.aptExact != nil {
				in.AptPackagesExact = &tc.aptExact
			}

			// Act
			opts := BuildOptions(in, rc)

			// Assert
			assert.ElementsMatch(t, tc.wantBases, opts.BaseOverride)
			assert.ElementsMatch(t, tc.wantApt, opts.AptPackages)
			assert.Equal(t, tc.basesExact != nil, opts.BaseExact)
			assert.Equal(t, tc.aptExact != nil, opts.AptExact)
			assert.Equal(t, len(tc.wantApt) > 0, opts.VerifyApt)
		})
	}
}

func TestUpdateOptions(t *testing.T) {
	rc := &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}, AptPackages: []string{"jq"}}}

	t.Run("config bases and apt are dropped without a flag", func(t *testing.T) {
		// Act
		opts := UpdateOptions(BuildInput{}, false, false, rc)

		// Assert - the image keeps what it was built with
		assert.Nil(t, opts.BaseOverride)
		assert.Nil(t, opts.AptPackages)
	})

	t.Run("config versions are dropped, flag versions kept", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{Versions: map[string]string{"java": "17", "node": "22"}}}

		// Act
		opts := UpdateOptions(BuildInput{VersionOverrides: map[string]string{"node": "24"}}, false, false, rc)

		// Assert
		assert.Equal(t, map[string]string{"node": "24"}, opts.Versions)
	})

	t.Run("a set but empty flag keeps the config values", func(t *testing.T) {
		// Act
		opts := UpdateOptions(BuildInput{}, true, true, rc)

		// Assert
		assert.Equal(t, []string{"java"}, opts.BaseOverride)
		assert.Equal(t, []string{"jq"}, opts.AptPackages)
	})
}
