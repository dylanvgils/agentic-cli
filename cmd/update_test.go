package cmd

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stubRunUpdateScript(t *testing.T, fn func(string, docker.BuildOptions) error) func() {
	t.Helper()
	orig := runUpdateScript
	runUpdateScript = fn
	return func() { runUpdateScript = orig }
}

func TestRunUpdate_allBuilt_updatesAll(t *testing.T) {
	// Arrange
	var updated []string
	restore := stubRunUpdateScript(t, func(tool string, _ docker.BuildOptions) error {
		updated = append(updated, tool)
		return nil
	})
	defer restore()

	restoreInspect := stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
	defer restoreInspect()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	// Act
	err := runUpdate(updateCmd, []string{})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"claude", "copilot", "opencode"}, updated)
}

func TestRunUpdate_allUnbuilt_skipsAll_printsMessage(t *testing.T) {
	// Arrange
	var updated []string
	restore := stubRunUpdateScript(t, func(tool string, _ docker.BuildOptions) error {
		updated = append(updated, tool)
		return nil
	})
	defer restore()

	restoreInspect := stubInspectImage(t, nil, nil)
	defer restoreInspect()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	// Act
	out := captureStdout(t, func() {
		err := runUpdate(updateCmd, []string{})
		require.NoError(t, err)
	})

	// Assert
	assert.Empty(t, updated)
	assert.Contains(t, out, "No tools are built.")
}

func TestRunUpdate_mixedBuilt_skipsUnbuilt(t *testing.T) {
	// Arrange
	var updated []string
	restore := stubRunUpdateScript(t, func(tool string, _ docker.BuildOptions) error {
		updated = append(updated, tool)
		return nil
	})
	defer restore()

	callCount := 0
	orig := inspectImage
	inspectImage = func(_ string) (*docker.ImageInfo, error) {
		callCount++
		if callCount == 1 {
			return nil, nil // claude not built
		}
		return &docker.ImageInfo{Version: "1.0.0"}, nil
	}
	defer func() { inspectImage = orig }()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	// Act
	out := captureStdout(t, func() {
		err := runUpdate(updateCmd, []string{})
		require.NoError(t, err)
	})

	// Assert
	assert.Equal(t, []string{"copilot", "opencode"}, updated)
	assert.Contains(t, out, "=> claude (skipped - not built)")
}

func TestRunUpdate_singleTool_alwaysUpdates(t *testing.T) {
	// Arrange
	var updated []string
	restore := stubRunUpdateScript(t, func(tool string, _ docker.BuildOptions) error {
		updated = append(updated, tool)
		return nil
	})
	defer restore()

	restoreInspect := stubInspectImage(t, nil, nil)
	defer restoreInspect()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	// Act
	err := runUpdate(updateCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"claude"}, updated)
}

func TestRunUpdate_versionChanged_reported(t *testing.T) {
	// Arrange
	restore := stubRunUpdateScript(t, func(_ string, _ docker.BuildOptions) error { return nil })
	defer restore()

	callCount := 0
	orig := inspectImage
	inspectImage = func(_ string) (*docker.ImageInfo, error) {
		callCount++
		if callCount == 1 {
			return &docker.ImageInfo{Version: "1.0.0"}, nil // before
		}
		return &docker.ImageInfo{Version: "2.0.0"}, nil // after
	}
	defer func() { inspectImage = orig }()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	// Act
	out := captureStdout(t, func() {
		err := runUpdate(updateCmd, []string{"claude"})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "=> version: 1.0.0 -> 2.0.0")
}

func TestRunUpdate_versionUpToDate_reported(t *testing.T) {
	// Arrange
	restore := stubRunUpdateScript(t, func(_ string, _ docker.BuildOptions) error { return nil })
	defer restore()

	restoreInspect := stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
	defer restoreInspect()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	// Act
	out := captureStdout(t, func() {
		err := runUpdate(updateCmd, []string{"claude"})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "=> version: 1.0.0 (up to date)")
}

func TestRunUpdate_scriptError_propagates(t *testing.T) {
	// Arrange
	restore := stubRunUpdateScript(t, func(_ string, _ docker.BuildOptions) error {
		return fmt.Errorf("docker daemon not running")
	})
	defer restore()

	restoreInspect := stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
	defer restoreInspect()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	// Act
	err := runUpdate(updateCmd, []string{"claude"})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker daemon not running")
}

func TestRunUpdate_stopsOnFirstToolError(t *testing.T) {
	// Arrange
	var updated []string
	restore := stubRunUpdateScript(t, func(tool string, _ docker.BuildOptions) error {
		updated = append(updated, tool)
		return fmt.Errorf("fail on %s", tool)
	})
	defer restore()

	restoreInspect := stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
	defer restoreInspect()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	// Act
	err := runUpdate(updateCmd, []string{})

	// Assert
	require.Error(t, err)
	assert.Len(t, updated, 1)
}

func TestRunUpdate_goFlag_setsOpt(t *testing.T) {
	// Arrange
	var capturedOpts docker.BuildOptions
	restore := stubRunUpdateScript(t, func(_ string, opts docker.BuildOptions) error {
		capturedOpts = opts
		return nil
	})
	defer restore()

	restoreInspect := stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
	defer restoreInspect()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	require.NoError(t, updateCmd.Flags().Set("go", "1.23"))
	defer updateCmd.Flags().Set("go", "") //nolint:errcheck

	// Act
	err := runUpdate(updateCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "1.23", capturedOpts.Versions["go"])
}

func TestRunUpdate_noCacheFlag_setsOpt(t *testing.T) {
	// Arrange
	var capturedOpts docker.BuildOptions
	restore := stubRunUpdateScript(t, func(_ string, opts docker.BuildOptions) error {
		capturedOpts = opts
		return nil
	})
	defer restore()

	restoreInspect := stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
	defer restoreInspect()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	require.NoError(t, updateCmd.Flags().Set("no-cache", "true"))
	defer updateCmd.Flags().Set("no-cache", "false") //nolint:errcheck

	// Act
	err := runUpdate(updateCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	assert.True(t, capturedOpts.NoCache)
}

func TestRunUpdate_pruneMessage_shown_whenReclaimedNonZero(t *testing.T) {
	// Arrange
	restore := stubRunUpdateScript(t, func(_ string, _ docker.BuildOptions) error { return nil })
	defer restore()

	restoreInspect := stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
	defer restoreInspect()

	restorePrune := stubPruneImages(t, func() (string, error) { return "512MB", nil })
	defer restorePrune()

	// Act
	out := captureStdout(t, func() {
		err := runUpdate(updateCmd, []string{"claude"})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "=> pruned dangling images (reclaimed 512MB)")
}
