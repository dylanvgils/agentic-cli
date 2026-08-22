package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAptPackages(t *testing.T) {
	t.Run("returns packages from rc", func(t *testing.T) {
		// Arrange
		rc := &AgenticRC{Build: RCBuild{AptPackages: []string{"make"}}}

		// Act
		result := AptPackages(rc)

		// Assert
		assert.Equal(t, []string{"make"}, result)
	})

	t.Run("env var appends to rc packages", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_APT_PACKAGES", "gcc")
		rc := &AgenticRC{Build: RCBuild{AptPackages: []string{"make"}}}

		// Act
		result := AptPackages(rc)

		// Assert
		assert.Equal(t, []string{"make", "gcc"}, result)
	})
}

func TestMarketplacesFor(t *testing.T) {
	t.Run("no tools filter matches every tool", func(t *testing.T) {
		// Arrange
		rc := &AgenticRC{Marketplaces: []RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}

		// Act
		result := MarketplacesFor(rc, "claude")

		// Assert
		assert.Equal(t, []RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git"}}, result)
	})

	t.Run("tools filter matches only listed tools", func(t *testing.T) {
		// Arrange
		rc := &AgenticRC{Marketplaces: []RCMarketplace{
			{Name: "claude-only", URL: "git@example.com:c.git", Tools: []string{"claude"}},
			{Name: "copilot-only", URL: "git@example.com:p.git", Tools: []string{"copilot"}},
		}}

		// Act
		result := MarketplacesFor(rc, "claude")

		// Assert
		assert.Equal(t, []RCMarketplace{{Name: "claude-only", URL: "git@example.com:c.git", Tools: []string{"claude"}}}, result)
	})
}

func TestCollectPaths(t *testing.T) {
	t.Run("no file", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()

		// Act
		paths := collectPaths(dir)

		// Assert
		assert.Empty(t, paths)
	})

	t.Run("in start dir", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rcPath := filepath.Join(dir, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(rcPath, []byte(""), 0o644))

		// Act
		paths := collectPaths(dir)

		// Assert
		assert.Equal(t, []string{rcPath}, paths)
	})

	t.Run("in parent dir", func(t *testing.T) {
		// Arrange
		parent := t.TempDir()
		child := filepath.Join(parent, "sub")
		require.NoError(t, os.Mkdir(child, 0o755))
		rcPath := filepath.Join(parent, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(rcPath, []byte(""), 0o644))

		// Act
		paths := collectPaths(child)

		// Assert - child first, then parent
		assert.Equal(t, []string{rcPath}, paths)
	})

	t.Run("multiple levels", func(t *testing.T) {
		// Arrange
		parent := t.TempDir()
		child := filepath.Join(parent, "sub")
		require.NoError(t, os.Mkdir(child, 0o755))
		parentRC := filepath.Join(parent, ".agenticrc.toml")
		childRC := filepath.Join(child, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(parentRC, []byte(""), 0o644))
		require.NoError(t, os.WriteFile(childRC, []byte(""), 0o644))

		// Act
		paths := collectPaths(child)

		// Assert - innermost first
		assert.Equal(t, []string{childRC, parentRC}, paths)
	})
}

func TestLoadConfigs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		// Act
		configs, err := loadConfigs(nil)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, configs)
	})

	t.Run("single file", func(t *testing.T) {
		// Arrange
		path := writeRC(t, "[run]\ncpus = \"4\"\n")

		// Act
		configs, err := loadConfigs([]string{path})

		// Assert
		require.NoError(t, err)
		require.Len(t, configs, 1)
		assert.Equal(t, "4", configs[0].Run.CPUs)
	})

	t.Run("stops at root true", func(t *testing.T) {
		// Arrange
		withRoot := writeRC(t, "root = true\n[run]\ncpus = \"4\"\n")
		shouldSkip := writeRC(t, "[run]\ncpus = \"1\"\n")

		// Act
		configs, err := loadConfigs([]string{withRoot, shouldSkip})

		// Assert - second file not loaded
		require.NoError(t, err)
		assert.Len(t, configs, 1)
		assert.Equal(t, "4", configs[0].Run.CPUs)
	})

	t.Run("returns error on invalid file and stops", func(t *testing.T) {
		// Arrange
		invalid := writeRC(t, "not valid toml [[[")
		valid := writeRC(t, "[run]\ncpus = \"4\"\n")

		// Act
		configs, err := loadConfigs([]string{invalid, valid})

		// Assert - error propagated, valid file after it not loaded
		assert.ErrorContains(t, err, invalid)
		assert.Empty(t, configs)
	})
}

func TestMergeConfigs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		// Act
		result := mergeConfigs(nil)

		// Assert
		assert.Empty(t, result.Run.ExtraMounts)
		assert.Empty(t, result.Run.Secrets)
		assert.Empty(t, result.Run.PidsLimit)
		assert.Empty(t, result.Run.CPUs)
		assert.Empty(t, result.Run.Memory)
	})

	t.Run("scalar child wins", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Run: RCRun{CPUs: "8", Memory: "8g"}}
		parent := &AgenticRC{Run: RCRun{CPUs: "2", Memory: "2g", PidsLimit: "512"}}

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
		child := &AgenticRC{Run: RCRun{ExtraMounts: []string{"child-vol:/mnt/c"}, Secrets: []string{"child-secret"}, Env: []string{"child-env=1"}}, Build: RCBuild{AptPackages: []string{"gcc"}, Bases: []string{"java"}}}
		parent := &AgenticRC{Run: RCRun{ExtraMounts: []string{"parent-vol:/mnt/p"}, Secrets: []string{"parent-secret"}, Env: []string{"parent-env=1"}}, Build: RCBuild{AptPackages: []string{"make"}, Bases: []string{"dotnet"}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert - parent (outermost) entries first
		assert.Equal(t, []string{"parent-vol:/mnt/p", "child-vol:/mnt/c"}, result.Run.ExtraMounts)
		assert.Equal(t, []string{"parent-secret", "child-secret"}, result.Run.Secrets)
		assert.Equal(t, []string{"parent-env=1", "child-env=1"}, result.Run.Env)
		assert.Equal(t, []string{"make", "gcc"}, result.Build.AptPackages)
		assert.Equal(t, []string{"dotnet", "java"}, result.Build.Bases)
	})

	t.Run("read_only_mounts accumulate outermost first", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Run: RCRun{ReadOnlyMounts: []string{"$PWD/child:/workspace/child"}}}
		parent := &AgenticRC{Run: RCRun{ReadOnlyMounts: []string{"$PWD/parent:/workspace/parent"}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, []string{"$PWD/parent:/workspace/parent", "$PWD/child:/workspace/child"}, result.Run.ReadOnlyMounts)
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

	t.Run("single config", func(t *testing.T) {
		// Arrange
		rc := &AgenticRC{Run: RCRun{CPUs: "4", ExtraMounts: []string{"vol:/mnt"}}}

		// Act
		result := mergeConfigs([]*AgenticRC{rc})

		// Assert
		assert.Equal(t, "4", result.Run.CPUs)
		assert.Equal(t, []string{"vol:/mnt"}, result.Run.ExtraMounts)
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

func TestParseRC(t *testing.T) {
	t.Run("all keys", func(t *testing.T) {
		// Arrange
		content := `
namespace = "myproject"

[build]
apt_packages = ["make", "gcc"]
bases = ["java", "dotnet"]

[build.versions]
node = "22"
java = "17"

[run]
extra_mounts = ["vol1:/mnt/a", "vol2:/mnt/b"]
secrets = ["token:/run/s/a", "key:/run/s/b"]
pids_limit = "512"
cpus = "2"
memory = "2g"
`
		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, []string{"vol1:/mnt/a", "vol2:/mnt/b"}, rc.Run.ExtraMounts)
		assert.Equal(t, []string{"token:/run/s/a", "key:/run/s/b"}, rc.Run.Secrets)
		assert.Equal(t, []string{"make", "gcc"}, rc.Build.AptPackages)
		assert.Equal(t, []string{"java", "dotnet"}, rc.Build.Bases)
		assert.Equal(t, map[string]string{"node": "22", "java": "17"}, rc.Build.Versions)
		assert.Equal(t, "512", rc.Run.PidsLimit)
		assert.Equal(t, "2", rc.Run.CPUs)
		assert.Equal(t, "2g", rc.Run.Memory)
		assert.Equal(t, "myproject", rc.Namespace)
	})

	t.Run("namespace key", func(t *testing.T) {
		// Act
		rc := mustParseRC(t, "namespace = \"work\"\n")

		// Assert
		assert.Equal(t, "work", rc.Namespace)
	})

	t.Run("root key", func(t *testing.T) {
		// Act + Assert
		assert.True(t, mustParseRC(t, "root = true\n").Root)
		assert.False(t, mustParseRC(t, "root = false\n").Root)
	})

	t.Run("comments and blanks", func(t *testing.T) {
		// Arrange
		content := "# this is a comment\n\n[run]\ncpus = \"4\"\n\n# another comment\nmemory = \"4g\"\n"

		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, "4", rc.Run.CPUs)
		assert.Equal(t, "4g", rc.Run.Memory)
		assert.Empty(t, rc.Run.ExtraMounts)
	})

	t.Run("tilde in string values", func(t *testing.T) {
		// Arrange
		content := "[run]\nextra_mounts = [\"~/.cache:/cache\"]\nsecrets = [\"mytoken:~/.secrets/token\"]\n"

		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, []string{"~/.cache:/cache"}, rc.Run.ExtraMounts)
		assert.Equal(t, []string{"mytoken:~/.secrets/token"}, rc.Run.Secrets)
	})

	t.Run("HOME env ref in string values", func(t *testing.T) {
		// Arrange
		content := "[run]\nextra_mounts = [\"$HOME/.cache:/cache\"]\nsecrets = [\"mytoken:${HOME}/.secrets/token\"]\n"

		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, []string{"$HOME/.cache:/cache"}, rc.Run.ExtraMounts)
		assert.Equal(t, []string{"mytoken:${HOME}/.secrets/token"}, rc.Run.Secrets)
	})

	t.Run("bases key", func(t *testing.T) {
		// Act
		rc := mustParseRC(t, "[build]\nbases = [\"java\", \"dotnet\"]\n")

		// Assert
		assert.Equal(t, []string{"java", "dotnet"}, rc.Build.Bases)
	})

	t.Run("versions key", func(t *testing.T) {
		// Act
		rc := mustParseRC(t, "[build.versions]\njava = \"17\"\nnode = \"22\"\n")

		// Assert
		assert.Equal(t, "17", rc.Build.Versions["java"])
		assert.Equal(t, "22", rc.Build.Versions["node"])
	})

	t.Run("proxy mode key", func(t *testing.T) {
		// Act
		rc := mustParseRC(t, "[run.proxy]\nmode = \"monitor\"\n")

		// Assert
		assert.Equal(t, ModeMonitor, rc.Run.Proxy.Mode)
	})

	t.Run("invalid proxy mode returns error with path", func(t *testing.T) {
		// Arrange
		path := writeRC(t, "[run.proxy]\nmode = \"bogus\"\n")

		// Act
		_, err := loadRC(path)

		// Assert
		assert.ErrorContains(t, err, "invalid [run.proxy] mode")
		assert.ErrorContains(t, err, path)
	})

	t.Run("run.instructions key", func(t *testing.T) {
		// Act
		rc := mustParseRC(t, "[run.instructions]\nenabled = false\ncustom = \"Always run go test before finishing.\"\n")

		// Assert
		require.NotNil(t, rc.Run.Instructions.Enabled)
		assert.False(t, *rc.Run.Instructions.Enabled)
		assert.Equal(t, "Always run go test before finishing.", rc.Run.Instructions.Custom)
	})

	t.Run("marketplaces key", func(t *testing.T) {
		// Act
		rc := mustParseRC(t, "[[marketplaces]]\nname = \"acme\"\nurl = \"git@example.com:acme.git\"\ntools = [\"claude\"]\n")

		// Assert
		assert.Equal(t, []RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git", Tools: []string{"claude"}}}, rc.Marketplaces)
	})

	t.Run("marketplaces key without tools filter", func(t *testing.T) {
		// Act
		rc := mustParseRC(t, "[[marketplaces]]\nname = \"acme\"\nurl = \"git@example.com:acme.git\"\n")

		// Assert
		assert.Equal(t, []RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git"}}, rc.Marketplaces)
	})

	t.Run("marketplace with empty name returns error with path", func(t *testing.T) {
		// Arrange
		path := writeRC(t, "[[marketplaces]]\nurl = \"git@example.com:acme.git\"\n")

		// Act
		_, err := loadRC(path)

		// Assert
		assert.ErrorContains(t, err, "name must not be empty")
		assert.ErrorContains(t, err, path)
	})

	t.Run("marketplace with unsafe name returns error with path", func(t *testing.T) {
		// Arrange
		path := writeRC(t, "[[marketplaces]]\nname = \"../escape\"\nurl = \"git@example.com:acme.git\"\n")

		// Act
		_, err := loadRC(path)

		// Assert
		assert.ErrorContains(t, err, "name must match")
		assert.ErrorContains(t, err, path)
	})

	t.Run("marketplace with empty url returns error with path", func(t *testing.T) {
		// Arrange
		path := writeRC(t, "[[marketplaces]]\nname = \"acme\"\n")

		// Act
		_, err := loadRC(path)

		// Assert
		assert.ErrorContains(t, err, "url must not be empty")
		assert.ErrorContains(t, err, path)
	})

	t.Run("custom_installs key", func(t *testing.T) {
		// Act
		rc := mustParseRC(t, "[[build.custom_installs]]\nname = \"helm\"\nrun = [\"curl -fsSL https://get.helm.sh -o /tmp/get-helm.sh\", \"bash /tmp/get-helm.sh\"]\n")

		// Assert
		assert.Equal(t, []RCCustomInstall{
			{Name: "helm", Run: []string{"curl -fsSL https://get.helm.sh -o /tmp/get-helm.sh", "bash /tmp/get-helm.sh"}},
		}, rc.Build.CustomInstalls)
	})

	t.Run("custom install with empty name returns error with path", func(t *testing.T) {
		// Arrange
		path := writeRC(t, "[[build.custom_installs]]\nrun = [\"true\"]\n")

		// Act
		_, err := loadRC(path)

		// Assert
		assert.ErrorContains(t, err, "name must not be empty")
		assert.ErrorContains(t, err, path)
	})

	t.Run("custom install with unsafe name returns error with path", func(t *testing.T) {
		// Arrange
		path := writeRC(t, "[[build.custom_installs]]\nname = \"../escape\"\nrun = [\"true\"]\n")

		// Act
		_, err := loadRC(path)

		// Assert
		assert.ErrorContains(t, err, "name must match")
		assert.ErrorContains(t, err, path)
	})

	t.Run("custom install with empty run returns error with path", func(t *testing.T) {
		// Arrange
		path := writeRC(t, "[[build.custom_installs]]\nname = \"helm\"\n")

		// Act
		_, err := loadRC(path)

		// Assert
		assert.ErrorContains(t, err, "run must not be empty")
		assert.ErrorContains(t, err, path)
	})

	t.Run("duplicate custom install name within one file returns error", func(t *testing.T) {
		// Arrange
		path := writeRC(t, "[[build.custom_installs]]\nname = \"helm\"\nrun = [\"true\"]\n\n[[build.custom_installs]]\nname = \"helm\"\nrun = [\"true\"]\n")

		// Act
		_, err := loadRC(path)

		// Assert
		assert.ErrorContains(t, err, "name must be unique")
		assert.ErrorContains(t, err, path)
	})

	t.Run("unknown key returns error with path", func(t *testing.T) {
		// Arrange
		content := "unknown = \"foo\"\ncpus = \"4\"\n"
		path := writeRC(t, content)

		// Act
		_, err := loadRC(path)

		// Assert
		assert.ErrorContains(t, err, "unknown keys")
		assert.ErrorContains(t, err, path)
	})

	t.Run("syntax error returns error with path", func(t *testing.T) {
		// Arrange
		path := writeRC(t, "not valid toml [[[")

		// Act
		_, err := loadRC(path)

		// Assert
		assert.ErrorContains(t, err, path)
	})

	t.Run("empty", func(t *testing.T) {
		// Act
		rc := mustParseRC(t, "")

		// Assert
		assert.Empty(t, rc.Run.ExtraMounts)
		assert.Empty(t, rc.Run.PidsLimit)
		assert.Empty(t, rc.Run.CPUs)
		assert.Empty(t, rc.Run.Memory)
	})
}

func TestFindLayers(t *testing.T) {
	t.Run("no files", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()

		// Act
		layers, err := FindLayers(dir)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, layers)
	})

	t.Run("single file", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rcPath := filepath.Join(dir, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(rcPath, []byte("[run]\ncpus = \"4\"\n"), 0o644))

		// Act
		layers, err := FindLayers(dir)

		// Assert
		require.NoError(t, err)
		require.Len(t, layers, 1)
		assert.Equal(t, rcPath, layers[0].Path)
		assert.Equal(t, "4", layers[0].RC.Run.CPUs)
	})

	t.Run("multiple files outermost first", func(t *testing.T) {
		// Arrange
		parent := t.TempDir()
		child := filepath.Join(parent, "sub")
		require.NoError(t, os.Mkdir(child, 0o755))
		parentRC := filepath.Join(parent, ".agenticrc.toml")
		childRC := filepath.Join(child, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(parentRC, []byte("[run]\ncpus = \"2\"\n"), 0o644))
		require.NoError(t, os.WriteFile(childRC, []byte("[run]\ncpus = \"8\"\n"), 0o644))

		// Act
		layers, err := FindLayers(child)

		// Assert - outermost (parent) is index 0
		require.NoError(t, err)
		require.Len(t, layers, 2)
		assert.Equal(t, parentRC, layers[0].Path)
		assert.Equal(t, childRC, layers[1].Path)
	})

	t.Run("stops at root", func(t *testing.T) {
		// Arrange
		grandparent := t.TempDir()
		parent := filepath.Join(grandparent, "mid")
		child := filepath.Join(parent, "sub")
		require.NoError(t, os.MkdirAll(child, 0o755))
		grandparentRC := filepath.Join(grandparent, ".agenticrc.toml")
		parentRC := filepath.Join(parent, ".agenticrc.toml")
		childRC := filepath.Join(child, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(grandparentRC, []byte("[run]\ncpus = \"1\"\n"), 0o644))
		require.NoError(t, os.WriteFile(parentRC, []byte("root = true\n[run]\ncpus = \"2\"\n"), 0o644))
		require.NoError(t, os.WriteFile(childRC, []byte("[run]\ncpus = \"8\"\n"), 0o644))

		// Act
		layers, err := FindLayers(child)

		// Assert - grandparent excluded because parent has root=true
		require.NoError(t, err)
		require.Len(t, layers, 2)
		assert.Equal(t, parentRC, layers[0].Path)
		assert.Equal(t, childRC, layers[1].Path)
	})

	t.Run("returns error on invalid file", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rcPath := filepath.Join(dir, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(rcPath, []byte("not valid toml [[["), 0o644))

		// Act
		layers, err := FindLayers(dir)

		// Assert
		assert.ErrorContains(t, err, rcPath)
		assert.Empty(t, layers)
	})
}

func TestFindAndLoadFromCwd(t *testing.T) {
	t.Run("no file returns empty config", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		orig, _ := os.Getwd()
		require.NoError(t, os.Chdir(dir))
		t.Cleanup(func() { _ = os.Chdir(orig) })

		// Act
		rc, err := FindAndLoadFromCwd()

		// Assert
		require.NoError(t, err)
		assert.Empty(t, rc.Run.CPUs)
		assert.Empty(t, rc.Run.ExtraMounts)
	})

	t.Run("invalid file returns error", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rcPath := filepath.Join(dir, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(rcPath, []byte("not valid toml [[["), 0o644))
		orig, _ := os.Getwd()
		require.NoError(t, os.Chdir(dir))
		t.Cleanup(func() { _ = os.Chdir(orig) })

		// Act
		rc, err := FindAndLoadFromCwd()

		// Assert
		assert.ErrorContains(t, err, rcPath)
		assert.Nil(t, rc)
	})
}

func TestFindAndLoad(t *testing.T) {
	t.Run("no file returns empty", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()

		// Act
		rc, err := FindAndLoad(dir)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, rc.Run.ExtraMounts)
		assert.Empty(t, rc.Run.Secrets)
		assert.Empty(t, rc.Run.PidsLimit)
		assert.Empty(t, rc.Run.CPUs)
		assert.Empty(t, rc.Run.Memory)
	})

	t.Run("merges from disk", func(t *testing.T) {
		// Arrange
		parent := t.TempDir()
		child := filepath.Join(parent, "sub")
		require.NoError(t, os.Mkdir(child, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(parent, ".agenticrc.toml"), []byte("root = true\n[run]\ncpus = \"2\"\nextra_mounts = [\"parent-vol:/mnt/p\"]\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(child, ".agenticrc.toml"), []byte("[run]\ncpus = \"8\"\nextra_mounts = [\"child-vol:/mnt/c\"]\n"), 0o644))

		// Act
		rc, err := FindAndLoad(child)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "8", rc.Run.CPUs)
		assert.Equal(t, []string{"parent-vol:/mnt/p", "child-vol:/mnt/c"}, rc.Run.ExtraMounts)
	})

	t.Run("broken outer layer fails the whole load even when inner layer is valid", func(t *testing.T) {
		// Arrange
		parent := t.TempDir()
		child := filepath.Join(parent, "sub")
		require.NoError(t, os.Mkdir(child, 0o755))
		parentRC := filepath.Join(parent, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(parentRC, []byte("not valid toml [[["), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(child, ".agenticrc.toml"), []byte("[run]\ncpus = \"8\"\n"), 0o644))

		// Act
		rc, err := FindAndLoad(child)

		// Assert
		assert.ErrorContains(t, err, parentRC)
		assert.Nil(t, rc)
	})

	t.Run("duplicate custom install name across layers returns error", func(t *testing.T) {
		// Arrange
		parent := t.TempDir()
		child := filepath.Join(parent, "sub")
		require.NoError(t, os.Mkdir(child, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(parent, ".agenticrc.toml"), []byte("[[build.custom_installs]]\nname = \"helm\"\nrun = [\"true\"]\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(child, ".agenticrc.toml"), []byte("[[build.custom_installs]]\nname = \"helm\"\nrun = [\"true\"]\n"), 0o644))

		// Act
		rc, err := FindAndLoad(child)

		// Assert
		assert.ErrorContains(t, err, "name must be unique across")
		assert.Nil(t, rc)
	})
}
