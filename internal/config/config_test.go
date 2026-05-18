package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_fileNotExist_returnsEmpty(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	cfg, err := LoadConfig(dir)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, cfg.TrustedDirs)
}

func TestLoadConfig_validFile_returnsParsed(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "agentic.json"),
		[]byte(`{"trusted_dirs":["/home/user/projects"]}`),
		0o640,
	))

	// Act
	cfg, err := LoadConfig(dir)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"/home/user/projects"}, cfg.TrustedDirs)
}

func TestLoadConfig_malformedJSON_returnsError(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "agentic.json"),
		[]byte(`not-json`),
		0o640,
	))

	// Act
	_, err := LoadConfig(dir)

	// Assert
	require.Error(t, err)
}

func TestSave_writesFileWithCorrectPerms(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	cfg := &CliConfig{TrustedDirs: []string{"/foo"}}

	// Act
	err := cfg.Save(dir)

	// Assert
	require.NoError(t, err)
	info, err := os.Stat(filepath.Join(dir, "agentic.json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())

	reloaded, err := LoadConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"/foo"}, reloaded.TrustedDirs)
}

func TestIsTrusted_exactMatch(t *testing.T) {
	// Arrange
	cfg := &CliConfig{TrustedDirs: []string{"/home/user/projects"}}

	// Act
	result := cfg.IsTrusted("/home/user/projects")

	// Assert
	assert.True(t, result)
}

func TestIsTrusted_parentMatch(t *testing.T) {
	// Arrange
	cfg := &CliConfig{TrustedDirs: []string{"/home/user/projects"}}

	// Act
	result := cfg.IsTrusted("/home/user/projects/foo")

	// Assert
	assert.True(t, result)
}

func TestIsTrusted_noMatch(t *testing.T) {
	// Arrange
	cfg := &CliConfig{TrustedDirs: []string{"/home/user/projects"}}

	// Act
	result := cfg.IsTrusted("/home/user/other")

	// Assert
	assert.False(t, result)
}

func TestIsTrusted_prefixWithoutSeparator_noMatch(t *testing.T) {
	// Arrange
	cfg := &CliConfig{TrustedDirs: []string{"/home/user/projects"}}

	// Act
	result := cfg.IsTrusted("/home/user/projects-evil")

	// Assert
	assert.False(t, result)
}

func TestIsTrusted_emptyConfig(t *testing.T) {
	// Act
	result := (&CliConfig{}).IsTrusted("/anything")

	// Assert
	assert.False(t, result)
}

func TestIsTrusted_symlinkDir_matches(t *testing.T) {
	// Arrange: create a real dir and a symlink pointing to it
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("cannot create symlink:", err)
	}
	cfg := &CliConfig{TrustedDirs: []string{real}}

	// Act
	result := cfg.IsTrusted(link)

	// Assert
	assert.True(t, result, "symlinked dir should be trusted when its target is trusted")
}

func TestTrust_appendsAndPersists(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	cfg := &CliConfig{}

	// Act
	err := cfg.Trust("/new/dir", dir)

	// Assert
	require.NoError(t, err)
	reloaded, err := LoadConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"/new/dir"}, reloaded.TrustedDirs)
}
