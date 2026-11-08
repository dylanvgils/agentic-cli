package config

import (
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
	for _, tc := range []struct {
		name   string
		parent bool
		child  bool
	}{
		{name: "no file"},
		{name: "in start dir", child: true},
		{name: "in parent dir", parent: true},
		{name: "innermost first across levels", parent: true, child: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			dirs := nestedDirs(t, 1)
			var want []string
			if tc.child {
				want = append(want, writeRCIn(t, dirs[1], ""))
			}
			if tc.parent {
				want = append(want, writeRCIn(t, dirs[0], ""))
			}

			// Act
			paths := collectPaths(dirs[1])

			// Assert
			assert.Equal(t, want, paths)
		})
	}
}

func TestParseRC(t *testing.T) {
	t.Run("parses every key without expanding paths", func(t *testing.T) {
		// Arrange
		content := `
root = true
namespace = "myproject"

[build]
apt_packages = ["make", "gcc"]
bases = ["java", "dotnet"]

[build.versions]
node = "22"
java = "17"

[[build.custom_installs]]
name = "helm"
run = ["curl -fsSL https://get.helm.sh -o /tmp/get-helm.sh", "bash /tmp/get-helm.sh"]

[run]
mounts = ["~/.cache:/cache", "$HOME/.m2:/m2"]
runtime = "microsandbox"
pids_limit = "512"
cpus = "2"
memory = "2g"

[run.args]
copilot = ["--disable-client-telemetry"]

[run.dind]
pids_limit = "2048"
cpus = "4"
memory = "8g"

[run.proxy]
mode = "monitor"
integrations = ["maven"]

[[run.proxy.credentials]]
preset = "example"
secret = "~/.example/secret"

[[run.proxy.credentials]]
hosts = ["api.example.test"]
header = "Authorization"
env = ["EXAMPLE_TOKEN"]
secret = "/example/secret"

[[run.proxy.allowed_paths]]
host = "raw.example.test"
paths = ["/org/repo/**", "/org/*/README.md"]

[run.instructions]
enabled = false
custom = "Always run go test before finishing."

[[marketplaces]]
name = "acme"
url = "git@example.com:acme.git"
tools = ["claude"]

[[marketplaces]]
name = "shared"
url = "git@example.com:shared.git"
`
		disabled := false

		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, &AgenticRC{
			Root:      true,
			Namespace: "myproject",
			Build: RCBuild{
				AptPackages: []string{"make", "gcc"},
				Bases:       []string{"java", "dotnet"},
				Versions:    map[string]string{"node": "22", "java": "17"},
				CustomInstalls: []RCCustomInstall{
					{Name: "helm", Run: []string{"curl -fsSL https://get.helm.sh -o /tmp/get-helm.sh", "bash /tmp/get-helm.sh"}},
				},
			},
			Run: RCRun{
				Mounts:   []string{"~/.cache:/cache", "$HOME/.m2:/m2"},
				Args:     map[string][]string{"copilot": {"--disable-client-telemetry"}},
				Runtime:  "microsandbox",
				RCLimits: RCLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"},
				Dind:     RCDind{RCLimits: RCLimits{PidsLimit: "2048", CPUs: "4", Memory: "8g"}},
				Proxy: RCProxy{
					Mode:         ModeMonitor,
					Integrations: []string{"maven"},
					Credentials: []RCCredential{
						{Preset: "example", Secret: "~/.example/secret"},
						{Hosts: []string{"api.example.test"}, Header: "Authorization", Env: []string{"EXAMPLE_TOKEN"}, Secret: "/example/secret"},
					},
					AllowedPaths: []RCAllowedPath{{Host: "raw.example.test", Paths: []string{"/org/repo/**", "/org/*/README.md"}}},
				},
				Instructions: RCInstructions{Enabled: &disabled, Custom: "Always run go test before finishing."},
			},
			Marketplaces: []RCMarketplace{
				{Name: "acme", URL: "git@example.com:acme.git", Tools: []string{"claude"}},
				{Name: "shared", URL: "git@example.com:shared.git"},
			},
		}, rc)
	})

	t.Run("empty", func(t *testing.T) {
		// Act
		rc := mustParseRC(t, "")

		// Assert
		assert.Equal(t, &AgenticRC{}, rc)
	})

	t.Run("unknown key returns error with path", func(t *testing.T) {
		// Arrange
		path := writeRC(t, "unknown = \"foo\"\ncpus = \"4\"\n")

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
}

func TestFindLayers(t *testing.T) {
	t.Run("no files", func(t *testing.T) {
		// Act
		layers, err := FindLayers(t.TempDir())

		// Assert
		require.NoError(t, err)
		assert.Empty(t, layers)
	})

	t.Run("parses each layer outermost first", func(t *testing.T) {
		// Arrange
		dirs := nestedDirs(t, 1)
		parentRC := writeRCIn(t, dirs[0], "[run]\ncpus = \"2\"\n")
		childRC := writeRCIn(t, dirs[1], "[run]\ncpus = \"8\"\n")

		// Act
		layers, err := FindLayers(dirs[1])

		// Assert
		require.NoError(t, err)
		require.Len(t, layers, 2)
		assert.Equal(t, parentRC, layers[0].Path)
		assert.Equal(t, "2", layers[0].RC.Run.CPUs)
		assert.Equal(t, childRC, layers[1].Path)
		assert.Equal(t, "8", layers[1].RC.Run.CPUs)
	})

	t.Run("stops at root", func(t *testing.T) {
		// Arrange
		dirs := nestedDirs(t, 2)
		writeRCIn(t, dirs[0], "[run]\ncpus = \"1\"\n")
		parentRC := writeRCIn(t, dirs[1], "root = true\n[run]\ncpus = \"2\"\n")
		childRC := writeRCIn(t, dirs[2], "[run]\ncpus = \"8\"\n")

		// Act
		layers, err := FindLayers(dirs[2])

		// Assert - grandparent excluded because parent has root=true
		require.NoError(t, err)
		require.Len(t, layers, 2)
		assert.Equal(t, parentRC, layers[0].Path)
		assert.Equal(t, childRC, layers[1].Path)
	})

	t.Run("returns error on invalid file", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rcPath := writeRCIn(t, dir, "not valid toml [[[")

		// Act
		layers, err := FindLayers(dir)

		// Assert
		assert.ErrorContains(t, err, rcPath)
		assert.Empty(t, layers)
	})
}

func TestLoadProject(t *testing.T) {
	t.Run("returns the work dir, its layers and their merge", func(t *testing.T) {
		// Arrange
		dirs := nestedDirs(t, 1)
		writeRCIn(t, dirs[0], `namespace = "outer"`)
		childRC := writeRCIn(t, dirs[1], `namespace = "inner"`)

		// Act
		project, err := LoadProject(dirs[1])

		// Assert
		require.NoError(t, err)
		assert.Equal(t, dirs[1], project.WorkDir)
		require.Len(t, project.Layers, 2)
		assert.Equal(t, childRC, project.Layers[1].Path)
		assert.Equal(t, "inner", project.RC.Namespace)
	})

	t.Run("invalid file returns error", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rcPath := writeRCIn(t, dir, "not valid toml [[[")

		// Act
		project, err := LoadProject(dir)

		// Assert
		assert.ErrorContains(t, err, rcPath)
		assert.Equal(t, Project{WorkDir: dir}, project)
	})
}

func TestFindAndLoad(t *testing.T) {
	t.Run("no file returns empty", func(t *testing.T) {
		// Act
		rc, err := FindAndLoad(t.TempDir())

		// Assert
		require.NoError(t, err)
		assert.Empty(t, rc.Run.Mounts)
		assert.Empty(t, rc.Run.RCLimits)
	})

	t.Run("merges from disk", func(t *testing.T) {
		// Arrange
		dirs := nestedDirs(t, 1)
		writeRCIn(t, dirs[0], "root = true\n[run]\ncpus = \"2\"\nmounts = [\"parent-vol:/mnt/p\"]\n")
		writeRCIn(t, dirs[1], "[run]\ncpus = \"8\"\nmounts = [\"child-vol:/mnt/c\"]\n")

		// Act
		rc, err := FindAndLoad(dirs[1])

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "8", rc.Run.CPUs)
		assert.Equal(t, []string{"parent-vol:/mnt/p", "child-vol:/mnt/c"}, rc.Run.Mounts)
	})

	t.Run("broken outer layer fails the whole load even when inner layer is valid", func(t *testing.T) {
		// Arrange
		dirs := nestedDirs(t, 1)
		parentRC := writeRCIn(t, dirs[0], "not valid toml [[[")
		writeRCIn(t, dirs[1], "[run]\ncpus = \"8\"\n")

		// Act
		rc, err := FindAndLoad(dirs[1])

		// Assert
		assert.ErrorContains(t, err, parentRC)
		assert.Nil(t, rc)
	})

	t.Run("duplicate custom install name across layers returns error", func(t *testing.T) {
		// Arrange
		dirs := nestedDirs(t, 1)
		writeRCIn(t, dirs[0], "[[build.custom_installs]]\nname = \"helm\"\nrun = [\"true\"]\n")
		writeRCIn(t, dirs[1], "[[build.custom_installs]]\nname = \"helm\"\nrun = [\"true\"]\n")

		// Act
		rc, err := FindAndLoad(dirs[1])

		// Assert
		assert.ErrorContains(t, err, "name must be unique across")
		assert.Nil(t, rc)
	})
}
