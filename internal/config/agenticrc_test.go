package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAptPackages(t *testing.T) {
	t.Run("returns packages from rc", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".agenticrc.toml"), []byte("[build]\napt_packages = [\"make\"]\n"), 0o644))

		// Act
		result := AptPackages(dir)

		// Assert
		assert.Equal(t, []string{"make"}, result)
	})

	t.Run("env var appends to rc packages", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_APT_PACKAGES", "gcc")
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".agenticrc.toml"), []byte("[build]\napt_packages = [\"make\"]\n"), 0o644))

		// Act
		result := AptPackages(dir)

		// Assert
		assert.Equal(t, []string{"make", "gcc"}, result)
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

		// Assert — child first, then parent
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

		// Assert — innermost first
		assert.Equal(t, []string{childRC, parentRC}, paths)
	})

	t.Run("legacy .agenticrc warns and is not collected", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		legacyPath := filepath.Join(dir, ".agenticrc")
		require.NoError(t, os.WriteFile(legacyPath, []byte("cpus=4\n"), 0o644))

		var buf bytes.Buffer
		orig := rcWarningWriter
		rcWarningWriter = &buf
		t.Cleanup(func() { rcWarningWriter = orig })

		// Act
		paths := collectPaths(dir)

		// Assert
		assert.Empty(t, paths)
		assert.Contains(t, buf.String(), legacyPath)
		assert.Contains(t, buf.String(), ".agenticrc.toml")
	})
}

func TestLoadConfigs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		// Act
		configs := loadConfigs(nil)

		// Assert
		assert.Empty(t, configs)
	})

	t.Run("single file", func(t *testing.T) {
		// Arrange
		path := writeRC(t, "[run]\ncpus = \"4\"\n")

		// Act
		configs := loadConfigs([]string{path})

		// Assert
		require.Len(t, configs, 1)
		assert.Equal(t, "4", configs[0].Run.CPUs)
	})

	t.Run("stops at root true", func(t *testing.T) {
		// Arrange
		withRoot := writeRC(t, "root = true\n[run]\ncpus = \"4\"\n")
		shouldSkip := writeRC(t, "[run]\ncpus = \"1\"\n")

		// Act
		configs := loadConfigs([]string{withRoot, shouldSkip})

		// Assert — second file not loaded
		assert.Len(t, configs, 1)
		assert.Equal(t, "4", configs[0].Run.CPUs)
	})

	t.Run("skips missing file", func(t *testing.T) {
		// Arrange
		valid := writeRC(t, "[run]\ncpus = \"4\"\n")

		// Act
		configs := loadConfigs([]string{"/nonexistent/.agenticrc.toml", valid})

		// Assert — missing file skipped, valid file loaded
		require.Len(t, configs, 1)
		assert.Equal(t, "4", configs[0].Run.CPUs)
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

		// Assert — child wins for set scalars, parent fills unset ones
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

	t.Run("lists accumulate outermost first", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Run: RCRun{ExtraMounts: []string{"child-vol:/mnt/c"}, Secrets: []string{"child-secret"}}, Build: RCBuild{AptPackages: []string{"gcc"}}}
		parent := &AgenticRC{Run: RCRun{ExtraMounts: []string{"parent-vol:/mnt/p"}, Secrets: []string{"parent-secret"}}, Build: RCBuild{AptPackages: []string{"make"}}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert — parent (outermost) entries first
		assert.Equal(t, []string{"parent-vol:/mnt/p", "child-vol:/mnt/c"}, result.Run.ExtraMounts)
		assert.Equal(t, []string{"parent-secret", "child-secret"}, result.Run.Secrets)
		assert.Equal(t, []string{"make", "gcc"}, result.Build.AptPackages)
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
}

func TestParseRC(t *testing.T) {
	t.Run("all keys", func(t *testing.T) {
		// Arrange
		content := `
namespace = "myproject"

[build]
apt_packages = ["make", "gcc"]

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

	t.Run("unknown key returns error", func(t *testing.T) {
		// Arrange
		content := "unknown = \"foo\"\ncpus = \"4\"\n"
		path := writeRC(t, content)

		// Act
		_, err := loadRC(path)

		// Assert
		assert.ErrorContains(t, err, "unknown keys")
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
		layers := FindLayers(dir)

		// Assert
		assert.Empty(t, layers)
	})

	t.Run("single file", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rcPath := filepath.Join(dir, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(rcPath, []byte("[run]\ncpus = \"4\"\n"), 0o644))

		// Act
		layers := FindLayers(dir)

		// Assert
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
		layers := FindLayers(child)

		// Assert — outermost (parent) is index 0
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
		layers := FindLayers(child)

		// Assert — grandparent excluded because parent has root=true
		require.Len(t, layers, 2)
		assert.Equal(t, parentRC, layers[0].Path)
		assert.Equal(t, childRC, layers[1].Path)
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
		rc := FindAndLoadFromCwd()

		// Assert
		assert.Empty(t, rc.Run.CPUs)
		assert.Empty(t, rc.Run.ExtraMounts)
	})
}

func TestFindAndLoad(t *testing.T) {
	t.Run("no file returns empty", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()

		// Act
		rc := FindAndLoad(dir)

		// Assert
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
		rc := FindAndLoad(child)

		// Assert
		assert.Equal(t, "8", rc.Run.CPUs)
		assert.Equal(t, []string{"parent-vol:/mnt/p", "child-vol:/mnt/c"}, rc.Run.ExtraMounts)
	})
}
