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
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".agenticrc"), []byte("apt_packages=make\n"), 0o644))

		// Act
		result := AptPackages(dir)

		// Assert
		assert.Equal(t, []string{"make"}, result)
	})

	t.Run("env var appends to rc packages", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_APT_PACKAGES", "gcc")
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".agenticrc"), []byte("apt_packages=make\n"), 0o644))

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
		rcPath := filepath.Join(dir, ".agenticrc")
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
		rcPath := filepath.Join(parent, ".agenticrc")
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
		parentRC := filepath.Join(parent, ".agenticrc")
		childRC := filepath.Join(child, ".agenticrc")
		require.NoError(t, os.WriteFile(parentRC, []byte(""), 0o644))
		require.NoError(t, os.WriteFile(childRC, []byte(""), 0o644))

		// Act
		paths := collectPaths(child)

		// Assert — innermost first
		assert.Equal(t, []string{childRC, parentRC}, paths)
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
		path := writeRC(t, "cpus=4\n")

		// Act
		configs := loadConfigs([]string{path})

		// Assert
		require.Len(t, configs, 1)
		assert.Equal(t, "4", configs[0].CPUs)
	})

	t.Run("stops at root true", func(t *testing.T) {
		// Arrange
		withRoot := writeRC(t, "root=true\ncpus=4\n")
		shouldSkip := writeRC(t, "cpus=1\n")

		// Act
		configs := loadConfigs([]string{withRoot, shouldSkip})

		// Assert — second file not loaded
		assert.Len(t, configs, 1)
		assert.Equal(t, "4", configs[0].CPUs)
	})

	t.Run("skips missing file", func(t *testing.T) {
		// Arrange
		valid := writeRC(t, "cpus=4\n")

		// Act
		configs := loadConfigs([]string{"/nonexistent/.agenticrc", valid})

		// Assert — missing file skipped, valid file loaded
		require.Len(t, configs, 1)
		assert.Equal(t, "4", configs[0].CPUs)
	})
}

func TestMergeConfigs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		// Act
		result := mergeConfigs(nil)

		// Assert
		assert.Empty(t, result.ExtraMounts)
		assert.Empty(t, result.Secrets)
		assert.Empty(t, result.PidsLimit)
		assert.Empty(t, result.CPUs)
		assert.Empty(t, result.Memory)
	})

	t.Run("scalar child wins", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{CPUs: "8", Memory: "8g"}
		parent := &AgenticRC{CPUs: "2", Memory: "2g", PidsLimit: "512"}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert — child wins for set scalars, parent fills unset ones
		assert.Equal(t, "8", result.CPUs)
		assert.Equal(t, "8g", result.Memory)
		assert.Equal(t, "512", result.PidsLimit)
	})

	t.Run("prefix child wins over parent", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{Prefix: "myproject"}
		parent := &AgenticRC{Prefix: "other"}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, "myproject", result.Prefix)
	})

	t.Run("prefix parent fills when child unset", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{}
		parent := &AgenticRC{Prefix: "shared"}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert
		assert.Equal(t, "shared", result.Prefix)
	})

	t.Run("lists accumulate outermost first", func(t *testing.T) {
		// Arrange
		child := &AgenticRC{ExtraMounts: []string{"child-vol:/mnt/c"}, Secrets: []string{"child-secret"}, AptPackages: []string{"gcc"}}
		parent := &AgenticRC{ExtraMounts: []string{"parent-vol:/mnt/p"}, Secrets: []string{"parent-secret"}, AptPackages: []string{"make"}}

		// Act
		result := mergeConfigs([]*AgenticRC{child, parent})

		// Assert — parent (outermost) entries first
		assert.Equal(t, []string{"parent-vol:/mnt/p", "child-vol:/mnt/c"}, result.ExtraMounts)
		assert.Equal(t, []string{"parent-secret", "child-secret"}, result.Secrets)
		assert.Equal(t, []string{"make", "gcc"}, result.AptPackages)
	})

	t.Run("single config", func(t *testing.T) {
		// Arrange
		rc := &AgenticRC{CPUs: "4", ExtraMounts: []string{"vol:/mnt"}}

		// Act
		result := mergeConfigs([]*AgenticRC{rc})

		// Assert
		assert.Equal(t, "4", result.CPUs)
		assert.Equal(t, []string{"vol:/mnt"}, result.ExtraMounts)
	})
}

func TestParseRC(t *testing.T) {
	t.Run("all keys", func(t *testing.T) {
		// Arrange
		content := "extra_mounts=vol1:/mnt/a,vol2:/mnt/b\nsecrets=token:/run/s/a,key:/run/s/b\napt_packages=make,gcc\npids_limit=512\ncpus=2\nmemory=2g\nprefix=myproject\n"

		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, []string{"vol1:/mnt/a", "vol2:/mnt/b"}, rc.ExtraMounts)
		assert.Equal(t, []string{"token:/run/s/a", "key:/run/s/b"}, rc.Secrets)
		assert.Equal(t, []string{"make", "gcc"}, rc.AptPackages)
		assert.Equal(t, "512", rc.PidsLimit)
		assert.Equal(t, "2", rc.CPUs)
		assert.Equal(t, "2g", rc.Memory)
		assert.Equal(t, "myproject", rc.Prefix)
	})

	t.Run("prefix key", func(t *testing.T) {
		// Act
		rc := mustParseRC(t, "prefix=work\n")

		// Assert
		assert.Equal(t, "work", rc.Prefix)
	})

	t.Run("repeatable keys", func(t *testing.T) {
		// Arrange
		content := "extra_mounts=vol1:/mnt/a\nextra_mounts=vol2:/mnt/b\nsecrets=gh-token\nsecrets=npm-token\napt_packages=make\napt_packages=gcc\n"

		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, []string{"vol1:/mnt/a", "vol2:/mnt/b"}, rc.ExtraMounts)
		assert.Equal(t, []string{"gh-token", "npm-token"}, rc.Secrets)
		assert.Equal(t, []string{"make", "gcc"}, rc.AptPackages)
	})

	t.Run("root key", func(t *testing.T) {
		// Act + Assert
		assert.True(t, mustParseRC(t, "root=true\n").Root)
		assert.False(t, mustParseRC(t, "root=false\n").Root)
	})

	t.Run("quoted values", func(t *testing.T) {
		// Arrange
		content := "pids_limit='1024'\ncpus=\"4\"\n"

		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, "1024", rc.PidsLimit)
		assert.Equal(t, "4", rc.CPUs)
	})

	t.Run("comments and blanks", func(t *testing.T) {
		// Arrange
		content := "# this is a comment\n\ncpus=4\n\n# another comment\nmemory=4g\n"

		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, "4", rc.CPUs)
		assert.Equal(t, "4g", rc.Memory)
		assert.Empty(t, rc.ExtraMounts)
	})

	t.Run("tilde expansion", func(t *testing.T) {
		// Arrange
		content := "extra_mounts=~/.cache:/cache\nsecrets=mytoken:~/.secrets/token\n"

		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, []string{"~/.cache:/cache"}, rc.ExtraMounts)
		assert.Equal(t, []string{"mytoken:~/.secrets/token"}, rc.Secrets)
	})

	t.Run("HOME env expansion", func(t *testing.T) {
		// Arrange
		content := "extra_mounts=$HOME/.cache:/cache\nsecrets=mytoken:${HOME}/.secrets/token\n"

		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, []string{"$HOME/.cache:/cache"}, rc.ExtraMounts)
		assert.Equal(t, []string{"mytoken:${HOME}/.secrets/token"}, rc.Secrets)
	})

	t.Run("unknown keys ignored", func(t *testing.T) {
		// Arrange
		content := "unknown=foo\ncpus=4\n"

		// Act
		rc := mustParseRC(t, content)

		// Assert
		assert.Equal(t, "4", rc.CPUs)
	})

	t.Run("empty", func(t *testing.T) {
		// Act
		rc := mustParseRC(t, "")

		// Assert
		assert.Empty(t, rc.ExtraMounts)
		assert.Empty(t, rc.PidsLimit)
		assert.Empty(t, rc.CPUs)
		assert.Empty(t, rc.Memory)
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
		rcPath := filepath.Join(dir, ".agenticrc")
		require.NoError(t, os.WriteFile(rcPath, []byte("cpus=4\n"), 0o644))

		// Act
		layers := FindLayers(dir)

		// Assert
		require.Len(t, layers, 1)
		assert.Equal(t, rcPath, layers[0].Path)
		assert.Equal(t, "4", layers[0].RC.CPUs)
	})

	t.Run("multiple files outermost first", func(t *testing.T) {
		// Arrange
		parent := t.TempDir()
		child := filepath.Join(parent, "sub")
		require.NoError(t, os.Mkdir(child, 0o755))
		parentRC := filepath.Join(parent, ".agenticrc")
		childRC := filepath.Join(child, ".agenticrc")
		require.NoError(t, os.WriteFile(parentRC, []byte("cpus=2\n"), 0o644))
		require.NoError(t, os.WriteFile(childRC, []byte("cpus=8\n"), 0o644))

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
		grandparentRC := filepath.Join(grandparent, ".agenticrc")
		parentRC := filepath.Join(parent, ".agenticrc")
		childRC := filepath.Join(child, ".agenticrc")
		require.NoError(t, os.WriteFile(grandparentRC, []byte("cpus=1\n"), 0o644))
		require.NoError(t, os.WriteFile(parentRC, []byte("root=true\ncpus=2\n"), 0o644))
		require.NoError(t, os.WriteFile(childRC, []byte("cpus=8\n"), 0o644))

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
		assert.Empty(t, rc.CPUs)
		assert.Empty(t, rc.ExtraMounts)
	})
}

func TestFindAndLoad(t *testing.T) {
	t.Run("no file returns empty", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()

		// Act
		rc := FindAndLoad(dir)

		// Assert
		assert.Empty(t, rc.ExtraMounts)
		assert.Empty(t, rc.Secrets)
		assert.Empty(t, rc.PidsLimit)
		assert.Empty(t, rc.CPUs)
		assert.Empty(t, rc.Memory)
	})

	t.Run("merges from disk", func(t *testing.T) {
		// Arrange
		parent := t.TempDir()
		child := filepath.Join(parent, "sub")
		require.NoError(t, os.Mkdir(child, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(parent, ".agenticrc"), []byte("root=true\ncpus=2\nextra_mounts=parent-vol:/mnt/p\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(child, ".agenticrc"), []byte("cpus=8\nextra_mounts=child-vol:/mnt/c\n"), 0o644))

		// Act
		rc := FindAndLoad(child)

		// Assert
		assert.Equal(t, "8", rc.CPUs)
		assert.Equal(t, []string{"parent-vol:/mnt/p", "child-vol:/mnt/c"}, rc.ExtraMounts)
	})
}
