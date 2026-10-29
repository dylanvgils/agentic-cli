package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
mounts = ["vol1:/mnt/a", "vol2:/mnt/b"]
pids_limit = "512"
cpus = "2"
memory = "2g"

[run.args]
copilot = ["--disable-client-telemetry"]
`
		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, []string{"vol1:/mnt/a", "vol2:/mnt/b"}, rc.Run.Mounts)
		assert.Equal(t, []string{"make", "gcc"}, rc.Build.AptPackages)
		assert.Equal(t, []string{"java", "dotnet"}, rc.Build.Bases)
		assert.Equal(t, map[string]string{"node": "22", "java": "17"}, rc.Build.Versions)
		assert.Equal(t, "512", rc.Run.PidsLimit)
		assert.Equal(t, "2", rc.Run.CPUs)
		assert.Equal(t, "2g", rc.Run.Memory)
		assert.Equal(t, map[string][]string{"copilot": {"--disable-client-telemetry"}}, rc.Run.Args)
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
		assert.Empty(t, rc.Run.Mounts)
	})

	t.Run("dind limits", func(t *testing.T) {
		// Arrange
		content := "[run]\ncpus = \"4\"\n\n[run.dind]\npids_limit = \"2048\"\ncpus = \"2\"\nmemory = \"8g\"\n"

		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, "4", rc.Run.CPUs)
		assert.Equal(t, "2048", rc.Run.Dind.PidsLimit)
		assert.Equal(t, "2", rc.Run.Dind.CPUs)
		assert.Equal(t, "8g", rc.Run.Dind.Memory)
	})

	t.Run("tilde in string values", func(t *testing.T) {
		// Arrange
		content := "[run]\nmounts = [\"~/.cache:/cache\"]\n"

		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, []string{"~/.cache:/cache"}, rc.Run.Mounts)
	})

	t.Run("HOME env ref in string values", func(t *testing.T) {
		// Arrange
		content := "[run]\nmounts = [\"$HOME/.cache:/cache\"]\n"

		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, []string{"$HOME/.cache:/cache"}, rc.Run.Mounts)
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

	t.Run("runtime key", func(t *testing.T) {
		// Act
		rc := mustParseRC(t, "[run]\nruntime = \"microsandbox\"\n")

		// Assert
		assert.Equal(t, "microsandbox", rc.Run.Runtime)
	})

	t.Run("proxy integrations key", func(t *testing.T) {
		// Act
		rc := mustParseRC(t, "[run.proxy]\nintegrations = [\"maven\"]\n")

		// Assert
		assert.Equal(t, []string{"maven"}, rc.Run.Proxy.Integrations)
	})

	t.Run("proxy credentials key", func(t *testing.T) {
		// Arrange
		content := `
[[run.proxy.credentials]]
preset = "example"
secret = "~/.example/secret"

[[run.proxy.credentials]]
hosts = ["api.example.test"]
header = "Authorization"
env = ["EXAMPLE_TOKEN"]
secret = "/example/secret"
`
		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, []RCCredential{
			{Preset: "example", Secret: "~/.example/secret"},
			{Hosts: []string{"api.example.test"}, Header: "Authorization", Env: []string{"EXAMPLE_TOKEN"}, Secret: "/example/secret"},
		}, rc.Run.Proxy.Credentials)
	})

	t.Run("proxy allowed_paths key", func(t *testing.T) {
		// Arrange
		content := `
[[run.proxy.allowed_paths]]
host = "raw.example.test"
paths = ["/org/repo/**", "/org/*/README.md"]
`
		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, []RCAllowedPath{{Host: "raw.example.test", Paths: []string{"/org/repo/**", "/org/*/README.md"}}}, rc.Run.Proxy.AllowedPaths)
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

	t.Run("custom_installs key", func(t *testing.T) {
		// Act
		rc := mustParseRC(t, "[[build.custom_installs]]\nname = \"helm\"\nrun = [\"curl -fsSL https://get.helm.sh -o /tmp/get-helm.sh\", \"bash /tmp/get-helm.sh\"]\n")

		// Assert
		assert.Equal(t, []RCCustomInstall{
			{Name: "helm", Run: []string{"curl -fsSL https://get.helm.sh -o /tmp/get-helm.sh", "bash /tmp/get-helm.sh"}},
		}, rc.Build.CustomInstalls)
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

	t.Run("invalid value returns error with path", func(t *testing.T) {
		// Arrange
		path := writeRC(t, "[run.dind]\npids_limit = \"-1\"\n")

		// Act
		_, err := loadRC(path)

		// Assert
		assert.EqualError(t, err, path+`: [run.dind] pids_limit "-1": must be a positive whole number`)
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
		assert.Empty(t, rc.Run.Mounts)
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
		assert.Empty(t, rc.Run.Mounts)
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
		assert.Empty(t, rc.Run.Mounts)
		assert.Empty(t, rc.Run.PidsLimit)
		assert.Empty(t, rc.Run.CPUs)
		assert.Empty(t, rc.Run.Memory)
	})

	t.Run("merges from disk", func(t *testing.T) {
		// Arrange
		parent := t.TempDir()
		child := filepath.Join(parent, "sub")
		require.NoError(t, os.Mkdir(child, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(parent, ".agenticrc.toml"), []byte("root = true\n[run]\ncpus = \"2\"\nmounts = [\"parent-vol:/mnt/p\"]\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(child, ".agenticrc.toml"), []byte("[run]\ncpus = \"8\"\nmounts = [\"child-vol:/mnt/c\"]\n"), 0o644))

		// Act
		rc, err := FindAndLoad(child)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "8", rc.Run.CPUs)
		assert.Equal(t, []string{"parent-vol:/mnt/p", "child-vol:/mnt/c"}, rc.Run.Mounts)
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
