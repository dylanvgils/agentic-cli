package agenticrc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeConfigs(t *testing.T) {
	enabled, disabled := true, false
	outer := &Config{
		Namespace:     "outer",
		DockerContext: "outer-context",
		Run: Run{
			Runtime:      "microsandbox",
			Proxy:        Proxy{Enabled: &enabled, Mode: ModeMonitor, Backend: BackendAgentic},
			Dind:         Dind{Enabled: &enabled, Limits: Limits{PidsLimit: "512", CPUs: "8", Memory: "8g"}},
			Instructions: Instructions{Enabled: &enabled},
			Limits:       Limits{PidsLimit: "512", CPUs: "2", Memory: "2g"},
			CheckUpdates: &enabled,
		},
	}

	t.Run("empty", func(t *testing.T) {
		// Act
		result := mergeConfigs(nil)

		// Assert
		assert.Equal(t, Config{}, scalarsOf(result))
		assert.Empty(t, result.Run.Mounts)
	})

	t.Run("inner scalars win, explicit false included", func(t *testing.T) {
		// Arrange
		inner := &Config{
			Namespace:     "inner",
			DockerContext: "inner-context",
			Run: Run{
				Runtime:      "docker",
				Proxy:        Proxy{Enabled: &disabled, Mode: ModeEnforce, Backend: BackendNative},
				Dind:         Dind{Enabled: &disabled, Limits: Limits{PidsLimit: "100", CPUs: "1", Memory: "1g"}},
				Instructions: Instructions{Enabled: &disabled},
				Limits:       Limits{PidsLimit: "200", CPUs: "4", Memory: "4g"},
				CheckUpdates: &disabled,
			},
		}

		// Act
		result := mergeConfigs([]*Config{inner, outer})

		// Assert
		assert.Equal(t, scalarsOf(inner), scalarsOf(result))
	})

	t.Run("outer scalars fill what inner leaves unset", func(t *testing.T) {
		// Act
		result := mergeConfigs([]*Config{{}, outer})

		// Assert
		assert.Equal(t, scalarsOf(outer), scalarsOf(result))
	})

	t.Run("run and dind limits merge per limit", func(t *testing.T) {
		// Arrange
		inner := &Config{Run: Run{
			Limits: Limits{CPUs: "4"},
			Dind:   Dind{Limits: Limits{CPUs: "1"}},
		}}

		// Act
		result := mergeConfigs([]*Config{inner, outer})

		// Assert
		assert.Equal(t, Limits{PidsLimit: "512", CPUs: "4", Memory: "2g"}, result.Run.Limits)
		assert.Equal(t, Limits{PidsLimit: "512", CPUs: "1", Memory: "8g"}, result.Run.Dind.Limits)
	})

	t.Run("lists accumulate outermost first", func(t *testing.T) {
		// Arrange
		inner := &Config{
			Build: Build{
				AptPackages:    []string{"gcc"},
				Bases:          []string{"java"},
				CustomInstalls: []CustomInstall{{Name: "golangci-lint", Run: []string{"echo inner"}}},
			},
			Run: Run{
				Mounts: []string{"inner-vol:/mnt/i"},
				Env:    []string{"inner-env=1"},
				Ports:  []string{"3000"},
				Args:   map[string][]string{"copilot": {"--inner"}},
				Proxy: Proxy{
					AllowedHosts: []string{"inner.example.test"},
					Credentials:  []Credential{{Preset: "inner-preset", Secret: "/inner"}},
					AllowedPaths: []AllowedPath{{Host: "inner.example.test", Paths: []string{"/inner"}}},
					Integrations: []string{"gradle"},
				},
			},
			Marketplaces: []Marketplace{{Name: "inner-mp", URL: "git@example.com:i.git"}},
		}
		outer := &Config{
			Build: Build{
				AptPackages:    []string{"make"},
				Bases:          []string{"dotnet"},
				CustomInstalls: []CustomInstall{{Name: "helm", Run: []string{"echo outer"}}},
			},
			Run: Run{
				Mounts: []string{"outer-vol:/mnt/o"},
				Env:    []string{"outer-env=1"},
				Ports:  []string{"8080:80"},
				Args:   map[string][]string{"copilot": {"--outer"}, "claude": {"--other"}},
				Proxy: Proxy{
					AllowedHosts: []string{"outer.example.test"},
					Credentials:  []Credential{{Preset: "outer-preset", Secret: "/outer"}},
					AllowedPaths: []AllowedPath{{Host: "outer.example.test", Paths: []string{"/outer"}}},
					Integrations: []string{"maven"},
				},
			},
			Marketplaces: []Marketplace{{Name: "outer-mp", URL: "git@example.com:o.git"}},
		}

		// Act
		result := mergeConfigs([]*Config{inner, outer})

		// Assert
		assert.Equal(t, Build{
			AptPackages: []string{"make", "gcc"},
			Bases:       []string{"dotnet", "java"},
			Versions:    map[string]string{},
			CustomInstalls: []CustomInstall{
				{Name: "helm", Run: []string{"echo outer"}},
				{Name: "golangci-lint", Run: []string{"echo inner"}},
			},
		}, result.Build)
		assert.Equal(t, []string{"outer-vol:/mnt/o", "inner-vol:/mnt/i"}, result.Run.Mounts)
		assert.Equal(t, []string{"outer-env=1", "inner-env=1"}, result.Run.Env)
		assert.Equal(t, []string{"8080:80", "3000"}, result.Run.Ports)
		assert.Equal(t, map[string][]string{"copilot": {"--outer", "--inner"}, "claude": {"--other"}}, result.Run.Args)
		assert.Equal(t, Proxy{
			AllowedHosts: []string{"outer.example.test", "inner.example.test"},
			Credentials:  []Credential{{Preset: "outer-preset", Secret: "/outer"}, {Preset: "inner-preset", Secret: "/inner"}},
			AllowedPaths: []AllowedPath{
				{Host: "outer.example.test", Paths: []string{"/outer"}},
				{Host: "inner.example.test", Paths: []string{"/inner"}},
			},
			Integrations: []string{"maven", "gradle"},
		}, result.Run.Proxy)
		assert.Equal(t, []Marketplace{
			{Name: "outer-mp", URL: "git@example.com:o.git"},
			{Name: "inner-mp", URL: "git@example.com:i.git"},
		}, result.Marketplaces)
	})

	t.Run("versions innermost wins per key", func(t *testing.T) {
		// Arrange
		inner := &Config{Build: Build{Versions: map[string]string{"node": "22", "java": "17"}}}
		outer := &Config{Build: Build{Versions: map[string]string{"node": "20", "dotnet": "8"}}}

		// Act
		result := mergeConfigs([]*Config{inner, outer})

		// Assert
		assert.Equal(t, map[string]string{"node": "22", "java": "17", "dotnet": "8"}, result.Build.Versions)
	})

	t.Run("empty inner version falls through to the outer value", func(t *testing.T) {
		// Arrange
		inner := &Config{Build: Build{Versions: map[string]string{"node": "", "java": ""}}}
		outer := &Config{Build: Build{Versions: map[string]string{"node": "22"}}}

		// Act
		result := mergeConfigs([]*Config{inner, outer})

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
			inner := &Config{Run: Run{Instructions: Instructions{Custom: tc.inner}}}
			outer := &Config{Run: Run{Instructions: Instructions{Custom: tc.outer}}}

			// Act
			result := mergeConfigs([]*Config{inner, outer})

			// Assert
			assert.Equal(t, tc.want, result.Run.Instructions.Custom)
		})
	}
}

func Test_mergeLimits(t *testing.T) {
	// Arrange
	parent := Limits{PidsLimit: "512", CPUs: "8", Memory: "8g"}
	limits := Limits{CPUs: "2"}

	// Act
	result := mergeLimits(limits, parent)

	// Assert - each limit merges on its own
	assert.Equal(t, Limits{PidsLimit: "512", CPUs: "2", Memory: "8g"}, result)
}

func Test_merge(t *testing.T) {
	t.Run("innermost scalar wins and lists accumulate outermost first", func(t *testing.T) {
		// Arrange
		layers := []Layer{
			{Path: "/outer", Config: &Config{Run: Run{
				Limits: Limits{CPUs: "2"},
				Proxy:  Proxy{Credentials: []Credential{{Preset: "github", Secret: "/example.test/gh"}}},
			}}},
			{Path: "/inner", Config: &Config{Run: Run{
				Limits: Limits{CPUs: "8"},
				Proxy:  Proxy{Credentials: []Credential{{Preset: "anthropic", Secret: "/example.test/key"}}},
			}}},
		}

		// Act
		rc, err := merge(layers)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "8", rc.Run.CPUs)
		require.Len(t, rc.Run.Proxy.Credentials, 2)
		assert.Equal(t, "github", rc.Run.Proxy.Credentials[0].Preset)
		assert.Equal(t, "anthropic", rc.Run.Proxy.Credentials[1].Preset)
	})

	t.Run("duplicate custom install name returns error", func(t *testing.T) {
		// Arrange
		install := CustomInstall{Name: "helm", Run: []string{"true"}}
		layers := []Layer{
			{Path: "/outer", Config: &Config{Build: Build{CustomInstalls: []CustomInstall{install}}}},
			{Path: "/inner", Config: &Config{Build: Build{CustomInstalls: []CustomInstall{install}}}},
		}

		// Act
		rc, err := merge(layers)

		// Assert
		assert.ErrorContains(t, err, "must be unique")
		assert.Nil(t, rc)
	})
}

func Test_fillUnset(t *testing.T) {
	disabled := false
	enabled := true

	for _, tc := range []struct {
		name string
		dst  string
		src  string
		want string
	}{
		{
			name: "fills an empty string",
			dst:  "",
			src:  "agentic",
			want: "agentic",
		},
		{
			name: "keeps a set string",
			dst:  "inner",
			src:  "outer",
			want: "inner",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			dst := tc.dst

			// Act
			fillUnset(&dst, tc.src)

			// Assert
			assert.Equal(t, tc.want, dst)
		})
	}

	for _, tc := range []struct {
		name string
		dst  *bool
		want *bool
	}{
		{
			name: "fills a nil pointer",
			dst:  nil,
			want: &enabled,
		},
		{
			name: "keeps an explicit false",
			dst:  &disabled,
			want: &disabled,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			dst := tc.dst

			// Act
			fillUnset(&dst, &enabled)

			// Assert
			assert.Same(t, tc.want, dst)
		})
	}
}
