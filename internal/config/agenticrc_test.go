package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- collectPaths ---

func TestCollectPaths_NoFile(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	paths := collectPaths(dir)

	// Assert
	assert.Empty(t, paths)
}

func TestCollectPaths_InStartDir(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	rcPath := filepath.Join(dir, ".agenticrc")
	require.NoError(t, os.WriteFile(rcPath, []byte(""), 0o644))

	// Act
	paths := collectPaths(dir)

	// Assert
	assert.Equal(t, []string{rcPath}, paths)
}

func TestCollectPaths_InParentDir(t *testing.T) {
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
}

func TestCollectPaths_MultipleLevels(t *testing.T) {
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
}

// --- loadConfigs ---

func TestLoadConfigs_Empty(t *testing.T) {
	// Act
	configs := loadConfigs(nil)

	// Assert
	assert.Empty(t, configs)
}

func TestLoadConfigs_SingleFile(t *testing.T) {
	// Arrange
	path := writeRC(t, "cpus=4\n")

	// Act
	configs := loadConfigs([]string{path})

	// Assert
	require.Len(t, configs, 1)
	assert.Equal(t, "4", configs[0].CPUs)
}

func TestLoadConfigs_StopsAtRootTrue(t *testing.T) {
	// Arrange
	withRoot := writeRC(t, "root=true\ncpus=4\n")
	shouldSkip := writeRC(t, "cpus=1\n")

	// Act
	configs := loadConfigs([]string{withRoot, shouldSkip})

	// Assert — second file not loaded
	assert.Len(t, configs, 1)
	assert.Equal(t, "4", configs[0].CPUs)
}

func TestLoadConfigs_SkipsMissingFile(t *testing.T) {
	// Arrange
	valid := writeRC(t, "cpus=4\n")

	// Act
	configs := loadConfigs([]string{"/nonexistent/.agenticrc", valid})

	// Assert — missing file skipped, valid file loaded
	require.Len(t, configs, 1)
	assert.Equal(t, "4", configs[0].CPUs)
}

// --- mergeConfigs ---

func TestMergeConfigs_Empty(t *testing.T) {
	// Act
	result := mergeConfigs(nil)

	// Assert
	assert.Empty(t, result.ExtraMounts)
	assert.Empty(t, result.Secrets)
	assert.Empty(t, result.PidsLimit)
	assert.Empty(t, result.CPUs)
	assert.Empty(t, result.Memory)
}

func TestMergeConfigs_ScalarChildWins(t *testing.T) {
	// Arrange
	child := &AgenticRC{CPUs: "8", Memory: "8g"}
	parent := &AgenticRC{CPUs: "2", Memory: "2g", PidsLimit: "512"}

	// Act
	result := mergeConfigs([]*AgenticRC{child, parent})

	// Assert — child wins for set scalars, parent fills unset ones
	assert.Equal(t, "8", result.CPUs)
	assert.Equal(t, "8g", result.Memory)
	assert.Equal(t, "512", result.PidsLimit)
}

func TestMergeConfigs_ListsAccumulateOutermostFirst(t *testing.T) {
	// Arrange
	child := &AgenticRC{ExtraMounts: []string{"child-vol:/mnt/c"}, Secrets: []string{"child-secret"}}
	parent := &AgenticRC{ExtraMounts: []string{"parent-vol:/mnt/p"}, Secrets: []string{"parent-secret"}}

	// Act
	result := mergeConfigs([]*AgenticRC{child, parent})

	// Assert — parent (outermost) entries first
	assert.Equal(t, []string{"parent-vol:/mnt/p", "child-vol:/mnt/c"}, result.ExtraMounts)
	assert.Equal(t, []string{"parent-secret", "child-secret"}, result.Secrets)
}

func TestMergeConfigs_SingleConfig(t *testing.T) {
	// Arrange
	rc := &AgenticRC{CPUs: "4", ExtraMounts: []string{"vol:/mnt"}}

	// Act
	result := mergeConfigs([]*AgenticRC{rc})

	// Assert
	assert.Equal(t, "4", result.CPUs)
	assert.Equal(t, []string{"vol:/mnt"}, result.ExtraMounts)
}

// --- parseRC ---

func TestParseRC_AllKeys(t *testing.T) {
	// Arrange
	content := "extra_mounts=vol1:/mnt/a,vol2:/mnt/b\nsecrets=token:/run/s/a,key:/run/s/b\npids_limit=512\ncpus=2\nmemory=2g\n"

	// Act
	rc := mustParseRC(t, content)

	// Assert
	assert.Equal(t, []string{"vol1:/mnt/a", "vol2:/mnt/b"}, rc.ExtraMounts)
	assert.Equal(t, []string{"token:/run/s/a", "key:/run/s/b"}, rc.Secrets)
	assert.Equal(t, "512", rc.PidsLimit)
	assert.Equal(t, "2", rc.CPUs)
	assert.Equal(t, "2g", rc.Memory)
}

func TestParseRC_RepeatableKeys(t *testing.T) {
	// Arrange
	content := "extra_mounts=vol1:/mnt/a\nextra_mounts=vol2:/mnt/b\nsecrets=gh-token\nsecrets=npm-token\n"

	// Act
	rc := mustParseRC(t, content)

	// Assert
	assert.Equal(t, []string{"vol1:/mnt/a", "vol2:/mnt/b"}, rc.ExtraMounts)
	assert.Equal(t, []string{"gh-token", "npm-token"}, rc.Secrets)
}

func TestParseRC_RootKey(t *testing.T) {
	// Act + Assert
	assert.True(t, mustParseRC(t, "root=true\n").Root)
	assert.False(t, mustParseRC(t, "root=false\n").Root)
}

func TestParseRC_QuotedValues(t *testing.T) {
	// Arrange
	content := "pids_limit='1024'\ncpus=\"4\"\n"

	// Act
	rc := mustParseRC(t, content)

	// Assert
	assert.Equal(t, "1024", rc.PidsLimit)
	assert.Equal(t, "4", rc.CPUs)
}

func TestParseRC_CommentsAndBlanks(t *testing.T) {
	// Arrange
	content := "# this is a comment\n\ncpus=4\n\n# another comment\nmemory=4g\n"

	// Act
	rc := mustParseRC(t, content)

	// Assert
	assert.Equal(t, "4", rc.CPUs)
	assert.Equal(t, "4g", rc.Memory)
	assert.Empty(t, rc.ExtraMounts)
}

func TestParseRC_TildeExpansion(t *testing.T) {
	// Arrange
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	content := "extra_mounts=~/.cache:/cache\nsecrets=mytoken:~/.secrets/token\n"

	// Act
	rc := mustParseRC(t, content)

	// Assert
	assert.Equal(t, []string{home + "/.cache:/cache"}, rc.ExtraMounts)
	assert.Equal(t, []string{"mytoken:" + home + "/.secrets/token"}, rc.Secrets)
}

func TestParseRC_HomeEnvExpansion(t *testing.T) {
	// Arrange
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	content := "extra_mounts=$HOME/.cache:/cache\nsecrets=mytoken:${HOME}/.secrets/token\n"

	// Act
	rc := mustParseRC(t, content)

	// Assert
	assert.Equal(t, []string{home + "/.cache:/cache"}, rc.ExtraMounts)
	assert.Equal(t, []string{"mytoken:" + home + "/.secrets/token"}, rc.Secrets)
}

func TestParseRC_UnknownKeysIgnored(t *testing.T) {
	// Arrange
	content := "unknown=foo\ncpus=4\n"

	// Act
	rc := mustParseRC(t, content)

	// Assert
	assert.Equal(t, "4", rc.CPUs)
}

func TestParseRC_Empty(t *testing.T) {
	// Act
	rc := mustParseRC(t, "")

	// Assert
	assert.Empty(t, rc.ExtraMounts)
	assert.Empty(t, rc.PidsLimit)
	assert.Empty(t, rc.CPUs)
	assert.Empty(t, rc.Memory)
}

// --- FindLayers ---
func TestFindLayers_NoFiles(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	layers := FindLayers(dir)

	// Assert
	assert.Empty(t, layers)
}

func TestFindLayers_SingleFile(t *testing.T) {
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
}

func TestFindLayers_MultipleFiles_OutermostFirst(t *testing.T) {
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
}

func TestFindLayers_StopsAtRoot(t *testing.T) {
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
}

// --- FindAndLoad (smoke tests) ---
func TestFindAndLoad_NoFile_ReturnsEmpty(t *testing.T) {
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
}

func TestFindAndLoad_MergesFromDisk(t *testing.T) {
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
}

// --- helpers ---
func mustParseRC(t *testing.T, content string) *AgenticRC {
	t.Helper()
	rc, err := parseRC(strings.NewReader(content))
	require.NoError(t, err)
	return rc
}

func writeRC(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".agenticrc")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}
