package cmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stubCleanImage(t *testing.T, fn func(string) error) func() {
	t.Helper()
	orig := cleanImage
	cleanImage = fn
	return func() { cleanImage = orig }
}

func stubCleanBaseImages(t *testing.T, fn func() error) func() {
	t.Helper()
	orig := cleanBaseImages
	cleanBaseImages = fn
	return func() { cleanBaseImages = orig }
}

func TestRunClean_allTools_whenNoArgs(t *testing.T) {
	// Arrange
	var cleaned []string
	restore := stubCleanImage(t, func(image string) error {
		cleaned = append(cleaned, image)
		return nil
	})
	defer restore()

	basesCleaned := false
	restoreBase := stubCleanBaseImages(t, func() error {
		basesCleaned = true
		return nil
	})
	defer restoreBase()

	// Act
	out := captureStdout(t, func() {
		err := runClean(cleanCmd, []string{})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "=> claude")
	assert.Contains(t, out, "=> copilot")
	assert.Contains(t, out, "=> opencode")
	assert.Contains(t, out, "=> base")
	assert.True(t, basesCleaned)
	assert.Len(t, cleaned, 3)
}

func TestRunClean_singleTool_whenArgGiven(t *testing.T) {
	// Arrange
	var cleaned []string
	restore := stubCleanImage(t, func(image string) error {
		cleaned = append(cleaned, image)
		return nil
	})
	defer restore()

	basesCleaned := false
	restoreBase := stubCleanBaseImages(t, func() error {
		basesCleaned = true
		return nil
	})
	defer restoreBase()

	// Act
	out := captureStdout(t, func() {
		err := runClean(cleanCmd, []string{"claude"})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "=> claude")
	assert.NotContains(t, out, "=> copilot")
	assert.NotContains(t, out, "=> opencode")
	assert.NotContains(t, out, "=> base")
	assert.False(t, basesCleaned)
	assert.Equal(t, []string{"agentic-claude"}, cleaned)
}

func TestRunClean_cleanImageError_propagates(t *testing.T) {
	// Arrange
	restore := stubCleanImage(t, func(_ string) error {
		return fmt.Errorf("docker daemon not running")
	})
	defer restore()

	restoreBase := stubCleanBaseImages(t, func() error { return nil })
	defer restoreBase()

	// Act
	err := runClean(cleanCmd, []string{"claude"})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker daemon not running")
}

func TestRunClean_cleanBaseImagesError_propagates(t *testing.T) {
	// Arrange
	restore := stubCleanImage(t, func(_ string) error { return nil })
	defer restore()

	restoreBase := stubCleanBaseImages(t, func() error {
		return fmt.Errorf("base cleanup failed")
	})
	defer restoreBase()

	// Act
	err := runClean(cleanCmd, []string{})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base cleanup failed")
}

func TestRunClean_stopsOnFirstToolError(t *testing.T) {
	// Arrange
	var cleaned []string
	restore := stubCleanImage(t, func(image string) error {
		cleaned = append(cleaned, image)
		return fmt.Errorf("fail on %s", image)
	})
	defer restore()

	restoreBase := stubCleanBaseImages(t, func() error { return nil })
	defer restoreBase()

	// Act
	err := runClean(cleanCmd, []string{})

	// Assert
	require.Error(t, err)
	assert.Len(t, cleaned, 1) // stopped after first failure
}
