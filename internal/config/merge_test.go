package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeConfigs(t *testing.T) {
	enabled, disabled := true, false
	outer := &AgenticRC{
		Namespace:     "outer",
		DockerContext: "outer-context",
		Run: RCRun{
			Runtime:      "microsandbox",
			Proxy:        RCProxy{Enabled: &enabled, Mode: ModeMonitor, Backend: BackendAgentic},
			Dind:         RCDind{Enabled: &enabled, RCLimits: RCLimits{PidsLimit: "512", CPUs: "8", Memory: "8g"}},
			Instructions: RCInstructions{Enabled: &enabled},
			RCLimits:     RCLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"},
			CheckUpdates: &enabled,
		},
	}

	t.Run("empty", func(t *testing.T) {
		// Act
		result := mergeConfigs(nil)

		// Assert
		assert.Equal(t, AgenticRC{}, scalarsOf(result))
		assert.Empty(t, result.Run.Mounts)
	})

	t.Run("inner scalars win, explicit false included", func(t *testing.T) {
		// Arrange
		inner := &AgenticRC{
			Namespace:     "inner",
			DockerContext: "inner-context",
			Run: RCRun{
				Runtime:      "docker",
				Proxy:        RCProxy{Enabled: &disabled, Mode: ModeEnforce, Backend: BackendNative},
				Dind:         RCDind{Enabled: &disabled, RCLimits: RCLimits{PidsLimit: "100", CPUs: "1", Memory: "1g"}},
				Instructions: RCInstructions{Enabled: &disabled},
				RCLimits:     RCLimits{PidsLimit: "200", CPUs: "4", Memory: "4g"},
				CheckUpdates: &disabled,
			},
		}

		// Act
		result := mergeConfigs([]*AgenticRC{inner, outer})

		// Assert
		assert.Equal(t, scalarsOf(inner), scalarsOf(result))
	})

	t.Run("outer scalars fill what inner leaves unset", func(t *testing.T) {
		// Act
		result := mergeConfigs([]*AgenticRC{{}, outer})

		// Assert
		assert.Equal(t, scalarsOf(outer), scalarsOf(result))
	})

	t.Run("run and dind limits merge per limit", func(t *testing.T) {
		// Arrange
		inner := &AgenticRC{Run: RCRun{
			RCLimits: RCLimits{CPUs: "4"},
			Dind:     RCDind{RCLimits: RCLimits{CPUs: "1"}},
		}}

		// Act
		result := mergeConfigs([]*AgenticRC{inner, outer})

		// Assert
		assert.Equal(t, RCLimits{PidsLimit: "512", CPUs: "4", Memory: "2g"}, result.Run.RCLimits)
		assert.Equal(t, RCLimits{PidsLimit: "512", CPUs: "1", Memory: "8g"}, result.Run.Dind.RCLimits)
	})

	t.Run("lists accumulate outermost first", func(t *testing.T) {
		// Arrange
		inner := &AgenticRC{
			Build: RCBuild{
				AptPackages:    []string{"gcc"},
				Bases:          []string{"java"},
				CustomInstalls: []RCCustomInstall{{Name: "golangci-lint", Run: []string{"echo inner"}}},
			},
			Run: RCRun{
				Mounts: []string{"inner-vol:/mnt/i"},
				Env:    []string{"inner-env=1"},
				Args:   map[string][]string{"copilot": {"--inner"}},
				Proxy: RCProxy{
					AllowedHosts: []string{"inner.example.test"},
					Credentials:  []RCCredential{{Preset: "inner-preset", Secret: "/inner"}},
					AllowedPaths: []RCAllowedPath{{Host: "inner.example.test", Paths: []string{"/inner"}}},
					Integrations: []string{"gradle"},
				},
			},
			Marketplaces: []RCMarketplace{{Name: "inner-mp", URL: "git@example.com:i.git"}},
		}
		outer := &AgenticRC{
			Build: RCBuild{
				AptPackages:    []string{"make"},
				Bases:          []string{"dotnet"},
				CustomInstalls: []RCCustomInstall{{Name: "helm", Run: []string{"echo outer"}}},
			},
			Run: RCRun{
				Mounts: []string{"outer-vol:/mnt/o"},
				Env:    []string{"outer-env=1"},
				Args:   map[string][]string{"copilot": {"--outer"}, "claude": {"--other"}},
				Proxy: RCProxy{
					AllowedHosts: []string{"outer.example.test"},
					Credentials:  []RCCredential{{Preset: "outer-preset", Secret: "/outer"}},
					AllowedPaths: []RCAllowedPath{{Host: "outer.example.test", Paths: []string{"/outer"}}},
					Integrations: []string{"maven"},
				},
			},
			Marketplaces: []RCMarketplace{{Name: "outer-mp", URL: "git@example.com:o.git"}},
		}

		// Act
		result := mergeConfigs([]*AgenticRC{inner, outer})

		// Assert
		assert.Equal(t, RCBuild{
			AptPackages: []string{"make", "gcc"},
			Bases:       []string{"dotnet", "java"},
			Versions:    map[string]string{},
			CustomInstalls: []RCCustomInstall{
				{Name: "helm", Run: []string{"echo outer"}},
				{Name: "golangci-lint", Run: []string{"echo inner"}},
			},
		}, result.Build)
		assert.Equal(t, []string{"outer-vol:/mnt/o", "inner-vol:/mnt/i"}, result.Run.Mounts)
		assert.Equal(t, []string{"outer-env=1", "inner-env=1"}, result.Run.Env)
		assert.Equal(t, map[string][]string{"copilot": {"--outer", "--inner"}, "claude": {"--other"}}, result.Run.Args)
		assert.Equal(t, RCProxy{
			AllowedHosts: []string{"outer.example.test", "inner.example.test"},
			Credentials:  []RCCredential{{Preset: "outer-preset", Secret: "/outer"}, {Preset: "inner-preset", Secret: "/inner"}},
			AllowedPaths: []RCAllowedPath{
				{Host: "outer.example.test", Paths: []string{"/outer"}},
				{Host: "inner.example.test", Paths: []string{"/inner"}},
			},
			Integrations: []string{"maven", "gradle"},
		}, result.Run.Proxy)
		assert.Equal(t, []RCMarketplace{
			{Name: "outer-mp", URL: "git@example.com:o.git"},
			{Name: "inner-mp", URL: "git@example.com:i.git"},
		}, result.Marketplaces)
	})

	t.Run("versions innermost wins per key", func(t *testing.T) {
		// Arrange
		inner := &AgenticRC{Build: RCBuild{Versions: map[string]string{"node": "22", "java": "17"}}}
		outer := &AgenticRC{Build: RCBuild{Versions: map[string]string{"node": "20", "dotnet": "8"}}}

		// Act
		result := mergeConfigs([]*AgenticRC{inner, outer})

		// Assert
		assert.Equal(t, map[string]string{"node": "22", "java": "17", "dotnet": "8"}, result.Build.Versions)
	})

	t.Run("empty inner version falls through to the outer value", func(t *testing.T) {
		// Arrange
		inner := &AgenticRC{Build: RCBuild{Versions: map[string]string{"node": "", "java": ""}}}
		outer := &AgenticRC{Build: RCBuild{Versions: map[string]string{"node": "22"}}}

		// Act
		result := mergeConfigs([]*AgenticRC{inner, outer})

		// Assert - empty values are unset, so java stays out of the map and build uses its default
		assert.Equal(t, map[string]string{"node": "22"}, result.Build.Versions)
	})

	for _, tc := range []struct {
		name  string
		inner string
		outer string
		want  string
	}{
		{name: "concatenates outermost first", inner: "inner rule", outer: "outer rule", want: "outer rule\n\ninner rule"},
		{name: "keeps inner text alone", inner: "inner rule", want: "inner rule"},
		{name: "keeps outer text alone", outer: "outer rule", want: "outer rule"},
	} {
		t.Run("instructions custom text "+tc.name, func(t *testing.T) {
			// Arrange
			inner := &AgenticRC{Run: RCRun{Instructions: RCInstructions{Custom: tc.inner}}}
			outer := &AgenticRC{Run: RCRun{Instructions: RCInstructions{Custom: tc.outer}}}

			// Act
			result := mergeConfigs([]*AgenticRC{inner, outer})

			// Assert
			assert.Equal(t, tc.want, result.Run.Instructions.Custom)
		})
	}
}

func Test_mergeLimits(t *testing.T) {
	parent := RCLimits{PidsLimit: "512", CPUs: "8", Memory: "8g"}

	t.Run("empty limits take the parent's", func(t *testing.T) {
		// Act
		result := mergeLimits(RCLimits{}, parent)

		// Assert
		assert.Equal(t, parent, result)
	})

	t.Run("set limits win over the parent", func(t *testing.T) {
		// Arrange
		limits := RCLimits{PidsLimit: "100", CPUs: "2", Memory: "2g"}

		// Act
		result := mergeLimits(limits, parent)

		// Assert
		assert.Equal(t, limits, result)
	})

	t.Run("each limit merges on its own", func(t *testing.T) {
		// Arrange
		limits := RCLimits{CPUs: "2"}

		// Act
		result := mergeLimits(limits, parent)

		// Assert
		assert.Equal(t, RCLimits{PidsLimit: "512", CPUs: "2", Memory: "8g"}, result)
	})
}

func TestMerge(t *testing.T) {
	t.Run("no layers returns empty", func(t *testing.T) {
		// Act
		rc, err := Merge(nil)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, rc.Run.Proxy.Credentials)
	})

	t.Run("innermost scalar wins and lists accumulate outermost first", func(t *testing.T) {
		// Arrange
		layers := []RCLayer{
			{Path: "/outer", RC: &AgenticRC{Run: RCRun{
				RCLimits: RCLimits{CPUs: "2"},
				Proxy:    RCProxy{Credentials: []RCCredential{{Preset: "github", Secret: "/example.test/gh"}}},
			}}},
			{Path: "/inner", RC: &AgenticRC{Run: RCRun{
				RCLimits: RCLimits{CPUs: "8"},
				Proxy:    RCProxy{Credentials: []RCCredential{{Preset: "anthropic", Secret: "/example.test/key"}}},
			}}},
		}

		// Act
		rc, err := Merge(layers)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "8", rc.Run.CPUs)
		require.Len(t, rc.Run.Proxy.Credentials, 2)
		assert.Equal(t, "github", rc.Run.Proxy.Credentials[0].Preset)
		assert.Equal(t, "anthropic", rc.Run.Proxy.Credentials[1].Preset)
	})

	t.Run("duplicate custom install name returns error", func(t *testing.T) {
		// Arrange
		install := RCCustomInstall{Name: "helm", Run: []string{"true"}}
		layers := []RCLayer{
			{Path: "/outer", RC: &AgenticRC{Build: RCBuild{CustomInstalls: []RCCustomInstall{install}}}},
			{Path: "/inner", RC: &AgenticRC{Build: RCBuild{CustomInstalls: []RCCustomInstall{install}}}},
		}

		// Act
		rc, err := Merge(layers)

		// Assert
		assert.ErrorContains(t, err, "must be unique")
		assert.Nil(t, rc)
	})
}

func Test_fillUnset(t *testing.T) {
	t.Run("fills an empty string", func(t *testing.T) {
		// Arrange
		dst := ""

		// Act
		fillUnset(&dst, "agentic")

		// Assert
		assert.Equal(t, "agentic", dst)
	})

	t.Run("keeps a set string", func(t *testing.T) {
		// Arrange
		dst := "inner"

		// Act
		fillUnset(&dst, "outer")

		// Assert
		assert.Equal(t, "inner", dst)
	})

	t.Run("fills a nil pointer", func(t *testing.T) {
		// Arrange
		var dst *bool
		enabled := true

		// Act
		fillUnset(&dst, &enabled)

		// Assert
		assert.Same(t, &enabled, dst)
	})

	t.Run("keeps an explicit false", func(t *testing.T) {
		// Arrange
		disabled := false
		enabled := true
		dst := &disabled

		// Act
		fillUnset(&dst, &enabled)

		// Assert
		assert.Same(t, &disabled, dst)
	})
}
