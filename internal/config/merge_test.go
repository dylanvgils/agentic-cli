package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeConfigs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		// Act
		result := mergeConfigs(nil)

		// Assert
		assert.Empty(t, result.Run.Mounts)
		assert.Empty(t, result.Run.PidsLimit)
		assert.Empty(t, result.Run.CPUs)
		assert.Empty(t, result.Run.Memory)
	})

	t.Run("scalar child wins", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Run: RCRun{RCLimits: RCLimits{CPUs: "8", Memory: "8g"}}}
		parent := &AgenticRC{Run: RCRun{RCLimits: RCLimits{CPUs: "2", Memory: "2g", PidsLimit: "512"}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert - child wins for set scalars, parent fills unset ones
		assert.Equal(t, "8", result.Run.CPUs)
		assert.Equal(t, "8g", result.Run.Memory)
		assert.Equal(t, "512", result.Run.PidsLimit)
	})

	t.Run("namespace child wins over parent", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Namespace: "myproject"}
		parent := &AgenticRC{Namespace: "other"}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, "myproject", result.Namespace)
	})

	t.Run("namespace parent fills when child unset", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{}
		parent := &AgenticRC{Namespace: "shared"}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, "shared", result.Namespace)
	})

	t.Run("docker_context child wins over parent", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{DockerContext: "prod"}
		parent := &AgenticRC{DockerContext: "other"}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, "prod", result.DockerContext)
	})

	t.Run("docker_context parent fills when child unset", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{}
		parent := &AgenticRC{DockerContext: "shared"}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, "shared", result.DockerContext)
	})

	t.Run("lists accumulate outermost first", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Run: RCRun{Mounts: []string{"child-vol:/mnt/c"}, Env: []string{"child-env=1"}}, Build: RCBuild{AptPackages: []string{"gcc"}, Bases: []string{"java"}}}
		parent := &AgenticRC{Run: RCRun{Mounts: []string{"parent-vol:/mnt/p"}, Env: []string{"parent-env=1"}}, Build: RCBuild{AptPackages: []string{"make"}, Bases: []string{"dotnet"}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert - parent (outermost) entries first
		assert.Equal(t, []string{"parent-vol:/mnt/p", "child-vol:/mnt/c"}, result.Run.Mounts)
		assert.Equal(t, []string{"parent-env=1", "child-env=1"}, result.Run.Env)
		assert.Equal(t, []string{"make", "gcc"}, result.Build.AptPackages)
		assert.Equal(t, []string{"dotnet", "java"}, result.Build.Bases)
	})

	t.Run("versions innermost wins per key", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Build: RCBuild{Versions: map[string]string{"node": "22", "java": "17"}}}
		parent := &AgenticRC{Build: RCBuild{Versions: map[string]string{"node": "20", "dotnet": "8"}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert - child wins for "node"; parent fills keys not set by child
		assert.Equal(t, "22", result.Build.Versions["node"])
		assert.Equal(t, "17", result.Build.Versions["java"])
		assert.Equal(t, "8", result.Build.Versions["dotnet"])
	})

	t.Run("empty inner version falls through to the outer value", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Build: RCBuild{Versions: map[string]string{"node": "", "java": ""}}}
		parent := &AgenticRC{Build: RCBuild{Versions: map[string]string{"node": "22"}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert - empty values are unset, so java stays out of the map and build uses its default
		assert.Equal(t, map[string]string{"node": "22"}, result.Build.Versions)
	})

	t.Run("single config", func(t *testing.T) {
		// Arrange
		rc := &AgenticRC{Run: RCRun{RCLimits: RCLimits{CPUs: "4"}, Mounts: []string{"vol:/mnt"}}}

		// Act
		result := mergeConfigs([]*AgenticRC{rc})

		// Assert
		assert.Equal(t, "4", result.Run.CPUs)
		assert.Equal(t, []string{"vol:/mnt"}, result.Run.Mounts)
	})

	t.Run("proxy allowed_hosts accumulate outermost first", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Run: RCRun{Proxy: RCProxy{AllowedHosts: []string{"child.example.com"}}}}
		parent := &AgenticRC{Run: RCRun{Proxy: RCProxy{AllowedHosts: []string{"parent.example.com"}}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, []string{"parent.example.com", "child.example.com"}, result.Run.Proxy.AllowedHosts)
	})

	t.Run("proxy integrations accumulate outermost first", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Run: RCRun{Proxy: RCProxy{Integrations: []string{"child"}}}}
		parent := &AgenticRC{Run: RCRun{Proxy: RCProxy{Integrations: []string{"parent"}}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, []string{"parent", "child"}, result.Run.Proxy.Integrations)
	})

	t.Run("proxy credentials accumulate outermost first", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Run: RCRun{Proxy: RCProxy{Credentials: []RCCredential{{Preset: "child-preset", Secret: "/child"}}}}}
		parent := &AgenticRC{Run: RCRun{Proxy: RCProxy{Credentials: []RCCredential{{Preset: "parent-preset", Secret: "/parent"}}}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, []RCCredential{{Preset: "parent-preset", Secret: "/parent"}, {Preset: "child-preset", Secret: "/child"}}, result.Run.Proxy.Credentials)
	})

	t.Run("args accumulate outermost first per tool", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Run: RCRun{Args: map[string][]string{"copilot": {"--child"}}}}
		parent := &AgenticRC{Run: RCRun{Args: map[string][]string{"copilot": {"--parent"}, "claude": {"--other"}}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, map[string][]string{"copilot": {"--parent", "--child"}, "claude": {"--other"}}, result.Run.Args)
	})

	t.Run("proxy allowed_paths accumulate outermost first", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Run: RCRun{Proxy: RCProxy{AllowedPaths: []RCAllowedPath{{Host: "child.example.test", Paths: []string{"/child"}}}}}}
		parent := &AgenticRC{Run: RCRun{Proxy: RCProxy{AllowedPaths: []RCAllowedPath{{Host: "parent.example.test", Paths: []string{"/parent"}}}}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, []RCAllowedPath{
			{Host: "parent.example.test", Paths: []string{"/parent"}},
			{Host: "child.example.test", Paths: []string{"/child"}},
		}, result.Run.Proxy.AllowedPaths)
	})

	t.Run("proxy enabled child wins over parent", func(t *testing.T) {
		// Arrange
		childFalse := false
		parentTrue := true
		child := &AgenticRC{Run: RCRun{Proxy: RCProxy{Enabled: &childFalse}}}
		parent := &AgenticRC{Run: RCRun{Proxy: RCProxy{Enabled: &parentTrue}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert - child explicitly disables, overriding the parent
		require.NotNil(t, result.Run.Proxy.Enabled)
		assert.False(t, *result.Run.Proxy.Enabled)
	})

	t.Run("proxy enabled parent fills when child unset", func(t *testing.T) {
		// Arrange
		parentTrue := true
		child := &AgenticRC{}
		parent := &AgenticRC{Run: RCRun{Proxy: RCProxy{Enabled: &parentTrue}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		require.NotNil(t, result.Run.Proxy.Enabled)
		assert.True(t, *result.Run.Proxy.Enabled)
	})

	t.Run("dind enabled child wins over parent", func(t *testing.T) {
		// Arrange
		childFalse := false
		parentTrue := true
		child := &AgenticRC{Run: RCRun{Dind: RCDind{Enabled: &childFalse}}}
		parent := &AgenticRC{Run: RCRun{Dind: RCDind{Enabled: &parentTrue}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		require.NotNil(t, result.Run.Dind.Enabled)
		assert.False(t, *result.Run.Dind.Enabled)
	})

	t.Run("dind enabled parent fills when child unset", func(t *testing.T) {
		// Arrange
		parentTrue := true
		child := &AgenticRC{}
		parent := &AgenticRC{Run: RCRun{Dind: RCDind{Enabled: &parentTrue}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		require.NotNil(t, result.Run.Dind.Enabled)
		assert.True(t, *result.Run.Dind.Enabled)
	})

	t.Run("dind limits child wins and parent fills gaps", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Run: RCRun{Dind: RCDind{RCLimits: RCLimits{CPUs: "2"}}}}
		parent := &AgenticRC{Run: RCRun{Dind: RCDind{RCLimits: RCLimits{PidsLimit: "512", CPUs: "8", Memory: "8g"}}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, RCDind{RCLimits: RCLimits{PidsLimit: "512", CPUs: "2", Memory: "8g"}}, result.Run.Dind)
	})

	t.Run("check_updates child wins over parent", func(t *testing.T) {
		// Arrange
		childFalse := false
		parentTrue := true
		child := &AgenticRC{Run: RCRun{CheckUpdates: &childFalse}}
		parent := &AgenticRC{Run: RCRun{CheckUpdates: &parentTrue}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert - child explicitly disables, overriding the parent
		require.NotNil(t, result.Run.CheckUpdates)
		assert.False(t, *result.Run.CheckUpdates)
	})

	t.Run("check_updates parent fills when child unset", func(t *testing.T) {
		// Arrange
		parentTrue := true
		child := &AgenticRC{}
		parent := &AgenticRC{Run: RCRun{CheckUpdates: &parentTrue}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		require.NotNil(t, result.Run.CheckUpdates)
		assert.True(t, *result.Run.CheckUpdates)
	})

	t.Run("proxy mode child wins over parent", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Run: RCRun{Proxy: RCProxy{Mode: ModeEnforce}}}
		parent := &AgenticRC{Run: RCRun{Proxy: RCProxy{Mode: ModeMonitor}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, ModeEnforce, result.Run.Proxy.Mode)
	})

	t.Run("proxy mode parent fills when child unset", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{}
		parent := &AgenticRC{Run: RCRun{Proxy: RCProxy{Mode: ModeMonitor}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, ModeMonitor, result.Run.Proxy.Mode)
	})

	t.Run("runtime child wins over parent", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Run: RCRun{Runtime: "docker"}}
		parent := &AgenticRC{Run: RCRun{Runtime: "microsandbox"}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, "docker", result.Run.Runtime)
	})

	t.Run("runtime parent fills when child unset", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{}
		parent := &AgenticRC{Run: RCRun{Runtime: "microsandbox"}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, "microsandbox", result.Run.Runtime)
	})

	t.Run("instructions enabled child wins over parent", func(t *testing.T) {
		// Arrange
		childFalse := false
		parentTrue := true
		child := &AgenticRC{Run: RCRun{Instructions: RCInstructions{Enabled: &childFalse}}}
		parent := &AgenticRC{Run: RCRun{Instructions: RCInstructions{Enabled: &parentTrue}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert - child explicitly disables, overriding the parent
		require.NotNil(t, result.Run.Instructions.Enabled)
		assert.False(t, *result.Run.Instructions.Enabled)
	})

	t.Run("instructions enabled parent fills when child unset", func(t *testing.T) {
		// Arrange
		parentTrue := true
		child := &AgenticRC{}
		parent := &AgenticRC{Run: RCRun{Instructions: RCInstructions{Enabled: &parentTrue}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		require.NotNil(t, result.Run.Instructions.Enabled)
		assert.True(t, *result.Run.Instructions.Enabled)
	})

	t.Run("instructions custom text concatenates outermost first", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Run: RCRun{Instructions: RCInstructions{Custom: "child rule"}}}
		parent := &AgenticRC{Run: RCRun{Instructions: RCInstructions{Custom: "parent rule"}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, "parent rule\n\nchild rule", result.Run.Instructions.Custom)
	})

	t.Run("instructions custom text child only", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Run: RCRun{Instructions: RCInstructions{Custom: "child rule"}}}
		parent := &AgenticRC{}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, "child rule", result.Run.Instructions.Custom)
	})

	t.Run("marketplaces accumulate outermost first", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Marketplaces: []RCMarketplace{{Name: "child-mp", URL: "git@example.com:c.git"}}}
		parent := &AgenticRC{Marketplaces: []RCMarketplace{{Name: "parent-mp", URL: "git@example.com:p.git"}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, []RCMarketplace{
			{Name: "parent-mp", URL: "git@example.com:p.git"},
			{Name: "child-mp", URL: "git@example.com:c.git"},
		}, result.Marketplaces)
	})

	t.Run("custom installs accumulate outermost first", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Build: RCBuild{CustomInstalls: []RCCustomInstall{{Name: "golangci-lint", Run: []string{"echo child"}}}}}
		parent := &AgenticRC{Build: RCBuild{CustomInstalls: []RCCustomInstall{{Name: "helm", Run: []string{"echo parent"}}}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, []RCCustomInstall{
			{Name: "helm", Run: []string{"echo parent"}},
			{Name: "golangci-lint", Run: []string{"echo child"}},
		}, result.Build.CustomInstalls)
	})
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
