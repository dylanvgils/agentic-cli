package cmd

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stubBuildTool(t *testing.T, fn func(tool, image, versionCmd string, opts docker.BuildOptions) error) func() {
	t.Helper()
	orig := buildTool
	buildTool = fn
	return func() { buildTool = orig }
}

func stubPruneImages(t *testing.T, fn func() (string, error)) func() {
	t.Helper()
	orig := pruneImages
	pruneImages = fn
	return func() { pruneImages = orig }
}

// dryRunBuild
func TestDryRunBuild_printsDockerfile_skipsScript(t *testing.T) {
	// Arrange
	var scriptCalled bool
	restore := stubBuildTool(t, func(_, _, _ string, _ docker.BuildOptions) error {
		scriptCalled = true
		return nil
	})
	defer restore()

	// Act
	out := captureStdout(t, func() {
		err := dryRunBuild([]string{"claude"}, docker.BuildOptions{Versions: map[string]string{}})
		require.NoError(t, err)
	})

	// Assert
	assert.False(t, scriptCalled)
	assert.Contains(t, out, "FROM")
}

// buildTools
func TestBuildTools_allTools_whenNoArgs(t *testing.T) {
	// Arrange
	var built []string
	restore := stubBuildTool(t, func(tool, _, _ string, _ docker.BuildOptions) error {
		built = append(built, tool)
		return nil
	})
	defer restore()

	// Act
	err := buildTools([]string{}, docker.BuildOptions{Versions: map[string]string{}})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"claude", "copilot", "opencode"}, built)
}

func TestBuildTools_singleTool_whenArgGiven(t *testing.T) {
	// Arrange
	var built []string
	restore := stubBuildTool(t, func(tool, _, _ string, _ docker.BuildOptions) error {
		built = append(built, tool)
		return nil
	})
	defer restore()

	// Act
	out := captureStdout(t, func() {
		err := buildTools([]string{"claude"}, docker.BuildOptions{Versions: map[string]string{}})
		require.NoError(t, err)
	})

	// Assert
	assert.Equal(t, []string{"claude"}, built)
	assert.Contains(t, out, "=> claude")
	assert.NotContains(t, out, "=> copilot")
	assert.NotContains(t, out, "=> opencode")
}

func TestBuildTools_scriptError_propagates(t *testing.T) {
	// Arrange
	restore := stubBuildTool(t, func(_, _, _ string, _ docker.BuildOptions) error {
		return fmt.Errorf("docker daemon not running")
	})
	defer restore()

	// Act
	err := buildTools([]string{"claude"}, docker.BuildOptions{Versions: map[string]string{}})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker daemon not running")
}

func TestBuildTools_stopsOnFirstToolError(t *testing.T) {
	// Arrange
	var built []string
	restore := stubBuildTool(t, func(tool, _, _ string, _ docker.BuildOptions) error {
		built = append(built, tool)
		return fmt.Errorf("fail on %s", tool)
	})
	defer restore()

	// Act
	err := buildTools([]string{}, docker.BuildOptions{Versions: map[string]string{}})

	// Assert
	require.Error(t, err)
	assert.Len(t, built, 1)
}

// runBuild
func TestRunBuild_noCacheFlag_setsOpt(t *testing.T) {
	// Arrange
	var capturedOpts docker.BuildOptions
	restore := stubBuildTool(t, func(_, _, _ string, opts docker.BuildOptions) error {
		capturedOpts = opts
		return nil
	})
	defer restore()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	require.NoError(t, buildCmd.Flags().Set("no-cache", "true"))
	defer buildCmd.Flags().Set("no-cache", "false") //nolint:errcheck

	// Act
	err := runBuild(buildCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	assert.True(t, capturedOpts.NoCache)
}

func TestRunBuild_baseFlag_setsOpt(t *testing.T) {
	// Arrange
	var capturedOpts docker.BuildOptions
	restore := stubBuildTool(t, func(_, _, _ string, opts docker.BuildOptions) error {
		capturedOpts = opts
		return nil
	})
	defer restore()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	require.NoError(t, buildCmd.Flags().Set("base", "java"))
	defer buildCmd.Flags().Set("base", "") //nolint:errcheck

	// Act
	err := runBuild(buildCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "java", capturedOpts.BaseOverride)
}

func TestRunBuild_nodeFlag_setsOpt(t *testing.T) {
	// Arrange
	var capturedOpts docker.BuildOptions
	restore := stubBuildTool(t, func(_, _, _ string, opts docker.BuildOptions) error {
		capturedOpts = opts
		return nil
	})
	defer restore()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	require.NoError(t, buildCmd.Flags().Set("node", "22"))
	defer buildCmd.Flags().Set("node", "") //nolint:errcheck

	// Act
	err := runBuild(buildCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "22", capturedOpts.NodeVersion)
}

func TestRunBuild_goFlag_setsOpt(t *testing.T) {
	// Arrange
	var capturedOpts docker.BuildOptions
	restore := stubBuildTool(t, func(_, _, _ string, opts docker.BuildOptions) error {
		capturedOpts = opts
		return nil
	})
	defer restore()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	require.NoError(t, buildCmd.Flags().Set("go", "1.23"))
	defer buildCmd.Flags().Set("go", "") //nolint:errcheck

	// Act
	err := runBuild(buildCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "1.23", capturedOpts.Versions["go"])
}

func TestRunBuild_pruneMessage_shown_whenReclaimedNonZero(t *testing.T) {
	// Arrange
	restore := stubBuildTool(t, func(_, _, _ string, _ docker.BuildOptions) error { return nil })
	defer restore()

	restorePrune := stubPruneImages(t, func() (string, error) { return "1.23GB", nil })
	defer restorePrune()

	// Act
	out := captureStdout(t, func() {
		err := runBuild(buildCmd, []string{"claude"})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "=> pruned dangling images (reclaimed 1.23GB)")
}

func TestRunBuild_pruneMessage_hidden_whenReclaimedZero(t *testing.T) {
	// Arrange
	restore := stubBuildTool(t, func(_, _, _ string, _ docker.BuildOptions) error { return nil })
	defer restore()

	restorePrune := stubPruneImages(t, func() (string, error) { return "", nil })
	defer restorePrune()

	// Act
	out := captureStdout(t, func() {
		err := runBuild(buildCmd, []string{"claude"})
		require.NoError(t, err)
	})

	// Assert
	assert.NotContains(t, out, "pruned dangling images")
}
