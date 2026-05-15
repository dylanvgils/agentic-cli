package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindRC_NotFound(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	_, ok := findRC(dir)

	// Assert
	assert.False(t, ok)
}

func TestFindRC_InStartDir(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	rcPath := filepath.Join(dir, ".agenticrc")
	require.NoError(t, os.WriteFile(rcPath, []byte(""), 0644))

	// Act
	found, ok := findRC(dir)

	// Assert
	assert.True(t, ok)
	assert.Equal(t, rcPath, found)
}

func TestFindRC_InParentDir(t *testing.T) {
	// Arrange
	parent := t.TempDir()
	child := filepath.Join(parent, "sub")
	require.NoError(t, os.Mkdir(child, 0755))
	rcPath := filepath.Join(parent, ".agenticrc")
	require.NoError(t, os.WriteFile(rcPath, []byte(""), 0644))

	// Act
	found, ok := findRC(child)

	// Assert
	assert.True(t, ok)
	assert.Equal(t, rcPath, found)
}

func TestFindAndLoad_NoFile_ReturnsEmpty(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	rc := FindAndLoad(dir)

	// Assert
	assert.Empty(t, rc.ExtraMounts)
	assert.Empty(t, rc.PidsLimit)
	assert.Empty(t, rc.CPUs)
	assert.Empty(t, rc.Memory)
}

func TestFindAndLoad_WithFile_ReturnsConfig(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".agenticrc"),
		[]byte("CPUS=8\nMEMORY=8g\n"),
		0644,
	))

	// Act
	rc := FindAndLoad(dir)

	// Assert
	assert.Equal(t, "8", rc.CPUs)
	assert.Equal(t, "8g", rc.Memory)
}

func TestLoadRC_AllKeys(t *testing.T) {
	// Arrange
	content := "EXTRA_MOUNTS=vol1:/mnt/a,vol2:/mnt/b\nPIDS_LIMIT=512\nCPUS=2\nMEMORY=2g\n"

	// Act
	rc := loadRCFromString(t, content)

	// Assert
	assert.Equal(t, []string{"vol1:/mnt/a", "vol2:/mnt/b"}, rc.ExtraMounts)
	assert.Equal(t, "512", rc.PidsLimit)
	assert.Equal(t, "2", rc.CPUs)
	assert.Equal(t, "2g", rc.Memory)
}

func TestLoadRC_QuotedValues(t *testing.T) {
	// Arrange
	content := "PIDS_LIMIT='1024'\nCPUS=\"4\"\n"

	// Act
	rc := loadRCFromString(t, content)

	// Assert
	assert.Equal(t, "1024", rc.PidsLimit)
	assert.Equal(t, "4", rc.CPUs)
}

func TestLoadRC_CommentsAndBlanks(t *testing.T) {
	// Arrange
	content := "# this is a comment\n\nCPUS=4\n\n# another comment\nMEMORY=4g\n"

	// Act
	rc := loadRCFromString(t, content)

	// Assert
	assert.Equal(t, "4", rc.CPUs)
	assert.Equal(t, "4g", rc.Memory)
	assert.Empty(t, rc.ExtraMounts)
}

func TestLoadRC_TildeExpansion(t *testing.T) {
	// Arrange
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	content := "EXTRA_MOUNTS=~/.cache:/cache\n"

	// Act
	rc := loadRCFromString(t, content)

	// Assert
	assert.Equal(t, []string{home + "/.cache:/cache"}, rc.ExtraMounts)
}

func TestLoadRC_UnknownKeysIgnored(t *testing.T) {
	// Arrange
	content := "UNKNOWN=foo\nCPUS=4\n"

	// Act
	rc := loadRCFromString(t, content)

	// Assert
	assert.Equal(t, "4", rc.CPUs)
}

func TestLoadRC_EmptyFile(t *testing.T) {
	// Arrange
	content := ""

	// Act
	rc := loadRCFromString(t, content)

	// Assert
	assert.Empty(t, rc.ExtraMounts)
	assert.Empty(t, rc.PidsLimit)
	assert.Empty(t, rc.CPUs)
	assert.Empty(t, rc.Memory)
}

func loadRCFromString(t *testing.T, content string) *AgenticRC {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".agenticrc")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	rc, err := loadRC(path)
	require.NoError(t, err)
	return rc
}
