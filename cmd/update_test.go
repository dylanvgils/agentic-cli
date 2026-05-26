package cmd

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stubUpdateTool(t *testing.T, fn func(tool, image string, opts tools.BuildOptions) error) func() {
	t.Helper()
	orig := updateTool
	updateTool = fn
	return func() { updateTool = orig }
}

// dryRunUpdate
func TestDryRunUpdate_printsDockerfile_skipsScript(t *testing.T) {
	// Arrange
	var scriptCalled bool
	restore := stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error {
		scriptCalled = true
		return nil
	})
	defer restore()

	// Act
	out := captureStdout(t, func() {
		err := dryRunUpdate([]string{"claude"}, tools.BuildOptions{Versions: map[string]string{}})
		require.NoError(t, err)
	})

	// Assert
	assert.False(t, scriptCalled)
	assert.Contains(t, out, "FROM")
}

func TestDryRunUpdate_withoutToolArg_returnsError(t *testing.T) {
	// Act
	err := dryRunUpdate([]string{}, tools.BuildOptions{Versions: map[string]string{}})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--dry-run requires a tool argument")
}

// updateTools
func TestUpdateTools_allBuilt_updatesAll(t *testing.T) {
	// Arrange
	var updated []string
	restore := stubUpdateTool(t, func(tool, _ string, _ tools.BuildOptions) error {
		updated = append(updated, tool)
		return nil
	})
	defer restore()

	restoreInspect := stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
	defer restoreInspect()

	// Act
	err := updateTools([]string{}, tools.BuildOptions{Versions: map[string]string{}})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"claude", "copilot", "opencode"}, updated)
}

func TestUpdateTools_allUnbuilt_skipsAll_printsMessage(t *testing.T) {
	// Arrange
	var updated []string
	restore := stubUpdateTool(t, func(tool, _ string, _ tools.BuildOptions) error {
		updated = append(updated, tool)
		return nil
	})
	defer restore()

	restoreInspect := stubInspectImage(t, nil, nil)
	defer restoreInspect()

	// Act
	out := captureStdout(t, func() {
		err := updateTools([]string{}, tools.BuildOptions{Versions: map[string]string{}})
		require.NoError(t, err)
	})

	// Assert
	assert.Empty(t, updated)
	assert.Contains(t, out, "No tools are built.")
}

func TestUpdateTools_mixedBuilt_skipsUnbuilt(t *testing.T) {
	// Arrange
	var updated []string
	restore := stubUpdateTool(t, func(tool, _ string, _ tools.BuildOptions) error {
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

	// Act
	out := captureStdout(t, func() {
		err := updateTools([]string{}, tools.BuildOptions{Versions: map[string]string{}})
		require.NoError(t, err)
	})

	// Assert
	assert.Equal(t, []string{"copilot", "opencode"}, updated)
	assert.Contains(t, out, "=> claude (skipped - not built)")
}

func TestUpdateTools_singleTool_alwaysUpdates(t *testing.T) {
	// Arrange
	var updated []string
	restore := stubUpdateTool(t, func(tool, _ string, _ tools.BuildOptions) error {
		updated = append(updated, tool)
		return nil
	})
	defer restore()

	restoreInspect := stubInspectImage(t, nil, nil)
	defer restoreInspect()

	// Act
	err := updateTools([]string{"claude"}, tools.BuildOptions{Versions: map[string]string{}})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"claude"}, updated)
}

func TestUpdateTools_stopsOnFirstToolError(t *testing.T) {
	// Arrange
	var updated []string
	restore := stubUpdateTool(t, func(tool, _ string, _ tools.BuildOptions) error {
		updated = append(updated, tool)
		return fmt.Errorf("fail on %s", tool)
	})
	defer restore()

	restoreInspect := stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
	defer restoreInspect()

	// Act
	err := updateTools([]string{}, tools.BuildOptions{Versions: map[string]string{}})

	// Assert
	require.Error(t, err)
	assert.Len(t, updated, 1)
}

// updateOneTool
func TestUpdateOneTool_versionChanged_reported(t *testing.T) {
	// Arrange
	restore := stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
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

	// Act
	out := captureStdout(t, func() {
		err := updateOneTool("claude", tools.BuildOptions{Versions: map[string]string{}})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "=> version: 1.0.0 -> 2.0.0")
}

func TestUpdateOneTool_versionUpToDate_reported(t *testing.T) {
	// Arrange
	restore := stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
	defer restore()

	restoreInspect := stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
	defer restoreInspect()

	// Act
	out := captureStdout(t, func() {
		err := updateOneTool("claude", tools.BuildOptions{Versions: map[string]string{}})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "=> version: 1.0.0 (up to date)")
}

func TestUpdateOneTool_scriptError_propagates(t *testing.T) {
	// Arrange
	restore := stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error {
		return fmt.Errorf("docker daemon not running")
	})
	defer restore()

	restoreInspect := stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
	defer restoreInspect()

	// Act
	err := updateOneTool("claude", tools.BuildOptions{Versions: map[string]string{}})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker daemon not running")
}

// runUpdate
func TestRunUpdate_baseFlag_setsOpt(t *testing.T) {
	// Arrange
	var capturedOpts tools.BuildOptions
	restore := stubUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
		capturedOpts = opts
		return nil
	})
	defer restore()

	restoreInspect := stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
	defer restoreInspect()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	require.NoError(t, updateCmd.Flags().Set("base", "java"))
	defer updateCmd.Flags().Set("base", "") //nolint:errcheck

	// Act
	err := runUpdate(updateCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "java", capturedOpts.BaseOverride)
}

func TestRunUpdate_nodeFlag_setsOpt(t *testing.T) {
	// Arrange
	var capturedOpts tools.BuildOptions
	restore := stubUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
		capturedOpts = opts
		return nil
	})
	defer restore()

	restoreInspect := stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
	defer restoreInspect()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	require.NoError(t, updateCmd.Flags().Set("node", "22"))
	defer updateCmd.Flags().Set("node", "") //nolint:errcheck

	// Act
	err := runUpdate(updateCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "22", capturedOpts.NodeVersion)
}

func TestRunUpdate_javaFlag_setsOpt(t *testing.T) {
	// Arrange
	var capturedOpts tools.BuildOptions
	restore := stubUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
		capturedOpts = opts
		return nil
	})
	defer restore()

	restoreInspect := stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
	defer restoreInspect()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	require.NoError(t, updateCmd.Flags().Set("java", "21"))
	defer updateCmd.Flags().Set("java", "") //nolint:errcheck

	// Act
	err := runUpdate(updateCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "21", capturedOpts.Versions["java"])
}

func TestRunUpdate_dotnetFlag_setsOpt(t *testing.T) {
	// Arrange
	var capturedOpts tools.BuildOptions
	restore := stubUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
		capturedOpts = opts
		return nil
	})
	defer restore()

	restoreInspect := stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
	defer restoreInspect()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	require.NoError(t, updateCmd.Flags().Set("dotnet", "10"))
	defer updateCmd.Flags().Set("dotnet", "") //nolint:errcheck

	// Act
	err := runUpdate(updateCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "10", capturedOpts.Versions["dotnet"])
}

func TestRunUpdate_goFlag_setsOpt(t *testing.T) {
	// Arrange
	var capturedOpts tools.BuildOptions
	restore := stubUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
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
	var capturedOpts tools.BuildOptions
	restore := stubUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
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
	restore := stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
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
