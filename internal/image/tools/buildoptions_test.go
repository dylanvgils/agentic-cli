package tools

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config/rc"
	"github.com/stretchr/testify/assert"
)

func Test_bases(t *testing.T) {
	agenticRC := &rc.AgenticRC{Build: rc.RCBuild{Bases: []string{"java"}}}

	for _, tc := range []struct {
		name  string
		flags []string
		rc    *rc.AgenticRC
		want  []string
	}{
		{name: "rc bases are included", rc: agenticRC, want: []string{"java"}},
		{name: "flag appends to rc bases in canonical order", flags: []string{"dotnet"}, rc: agenticRC, want: []string{"dotnet", "java"}},
		{name: "empty when no sources set", rc: &rc.AgenticRC{}, want: []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := bases(tc.flags, tc.rc)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func Test_basesExact(t *testing.T) {
	// Act
	result := basesExact([]string{"java", "dotnet"})

	// Assert - sorted by canonical extras order
	assert.Equal(t, []string{"dotnet", "java"}, result)
}

func Test_versions(t *testing.T) {
	agenticRC := &rc.AgenticRC{Build: rc.RCBuild{Versions: map[string]string{"java": "17"}}}

	for _, tc := range []struct {
		name  string
		flags map[string]string
		rc    *rc.AgenticRC
		want  map[string]string
	}{
		{name: "rc versions used as defaults", rc: agenticRC, want: map[string]string{"java": "17"}},
		{name: "flag overrides rc version", flags: map[string]string{"java": "21"}, rc: agenticRC, want: map[string]string{"java": "21"}},
		{name: "empty flag value keeps rc version", flags: map[string]string{"java": ""}, rc: agenticRC, want: map[string]string{"java": "17"}},
		{name: "empty when no sources set", rc: &rc.AgenticRC{}, want: map[string]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := versions(tc.flags, tc.rc)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func Test_aptPackages(t *testing.T) {
	agenticRC := &rc.AgenticRC{Build: rc.RCBuild{AptPackages: []string{"make"}}}

	for _, tc := range []struct {
		name  string
		flags []string
		rc    *rc.AgenticRC
		want  []string
	}{
		{name: "rc packages are included", rc: agenticRC, want: []string{"make"}},
		{name: "flag appends to config packages", flags: []string{"gcc"}, rc: agenticRC, want: []string{"make", "gcc"}},
		{name: "empty when no sources set", rc: &rc.AgenticRC{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := aptPackages(tc.flags, tc.rc)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func Test_aptPackagesExact(t *testing.T) {
	// Act
	result := aptPackagesExact([]string{"make", "gcc", "make"})

	// Assert
	assert.Equal(t, []string{"make", "gcc"}, result)
}

func TestBuildOptionsFor(t *testing.T) {
	t.Run("merges the flags with rc", func(t *testing.T) {
		// Arrange
		agenticRC := &rc.AgenticRC{Build: rc.RCBuild{
			Bases:          []string{"java"},
			AptPackages:    []string{"jq"},
			Versions:       map[string]string{"java": "17"},
			CustomInstalls: []rc.RCCustomInstall{{Name: "foo"}},
		}}
		in := BuildFlags{
			Bases:               []string{"dotnet"},
			VersionOverrides:    map[string]string{"node": "24"},
			AptPackages:         []string{"make"},
			NoCache:             true,
			Pull:                true,
			Registry:            "registry.example.test",
			SkipInstallChecksum: true,
		}

		// Act
		opts := BuildOptionsFor(in, agenticRC)

		// Assert
		assert.Equal(t, BuildOptions{
			BaseOverride:        []string{"dotnet", "java"},
			Versions:            map[string]string{"java": "17", "node": "24"},
			AptPackages:         []string{"jq", "make"},
			VerifyApt:           true,
			NoCache:             true,
			Pull:                true,
			Registry:            "registry.example.test",
			CustomInstalls:      agenticRC.Build.CustomInstalls,
			SkipInstallChecksum: true,
		}, opts)
	})

	agenticRC := &rc.AgenticRC{Build: rc.RCBuild{Bases: []string{"java"}, AptPackages: []string{"make"}}}

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
			var in BuildFlags
			if tc.basesExact != nil {
				in.BasesExact = &tc.basesExact
			}
			if tc.aptExact != nil {
				in.AptPackagesExact = &tc.aptExact
			}

			// Act
			opts := BuildOptionsFor(in, agenticRC)

			// Assert
			assert.ElementsMatch(t, tc.wantBases, opts.BaseOverride)
			assert.ElementsMatch(t, tc.wantApt, opts.AptPackages)
			assert.Equal(t, tc.basesExact != nil, opts.BaseExact)
			assert.Equal(t, tc.aptExact != nil, opts.AptExact)
			assert.Equal(t, len(tc.wantApt) > 0, opts.VerifyApt)
		})
	}
}

func TestUpdateOptionsFor(t *testing.T) {
	agenticRC := &rc.AgenticRC{Build: rc.RCBuild{Bases: []string{"java"}, AptPackages: []string{"jq"}}}

	t.Run("config bases and apt are dropped without a flag", func(t *testing.T) {
		// Act
		opts := UpdateOptionsFor(BuildFlags{}, false, false, agenticRC)

		// Assert - the image keeps what it was built with
		assert.Nil(t, opts.BaseOverride)
		assert.Nil(t, opts.AptPackages)
	})

	t.Run("config versions are dropped, flag versions kept", func(t *testing.T) {
		// Arrange
		agenticRC := &rc.AgenticRC{Build: rc.RCBuild{Versions: map[string]string{"java": "17", "node": "22"}}}

		// Act
		opts := UpdateOptionsFor(BuildFlags{VersionOverrides: map[string]string{"node": "24"}}, false, false, agenticRC)

		// Assert
		assert.Equal(t, map[string]string{"node": "24"}, opts.Versions)
	})

	t.Run("a set but empty flag keeps the config values", func(t *testing.T) {
		// Act
		opts := UpdateOptionsFor(BuildFlags{}, true, true, agenticRC)

		// Assert
		assert.Equal(t, []string{"java"}, opts.BaseOverride)
		assert.Equal(t, []string{"jq"}, opts.AptPackages)
	})
}
