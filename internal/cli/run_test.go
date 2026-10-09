package cli

import (
	"slices"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runTool(t *testing.T) {
	// Keep the repo's own .agenticrc.toml out of the test
	t.Chdir(t.TempDir())

	t.Run("no args prints help", func(t *testing.T) {
		// Arrange
		get := captureRunContainer(t)

		// Act
		err := runTool(runToolCmd, []string{})

		// Assert
		require.NoError(t, err)
		rs, _ := get()
		assert.Empty(t, rs.Image, "RunContainer should not be called when no args given")
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		err := runTool(runToolCmd, []string{"bogus"})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bogus")
	})

	t.Run("builds image name", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		withTempToolHome(t)
		get := captureRunContainer(t)

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		rs, toolArgs := get()
		assert.Equal(t, "agentic-claude", rs.Image)
		assert.Empty(t, toolArgs)
	})

	t.Run("read-only-mount flag forces the mount :ro and appends it last", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		withTempToolHome(t)
		get := captureRunContainer(t)
		stubFlag(t, runToolCmd, "read-only-mount", "$PWD/creds:/workspace/creds")

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		rs, _ := get()
		require.Contains(t, rs.Volumes, "$PWD/creds:/workspace/creds:ro")
		assert.Equal(t, len(rs.Volumes)-1, slices.Index(rs.Volumes, "$PWD/creds:/workspace/creds:ro"),
			"flag-supplied read-only mount must be last so it shadows every other mount")
	})

	t.Run("dind flag enables the sidecar when the image has the docker layer", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		withTempToolHome(t)
		get := captureRunContainer(t)
		stubInspectImage(t, &docker.ImageInfo{Image: "agentic-claude", Base: "docker@29.8.2"}, nil)
		stubDocker(t, &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Image: "agentic-claude", Base: "docker@29.8.2"}, nil)})
		stubBuildDindImage(t, func(string, tools.BuildOptions) error { return nil })
		stubFlag(t, runToolCmd, "dind", "true")

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		rs, _ := get()
		assert.True(t, rs.Dind.Enabled)
		assert.Equal(t, tools.DindImage, rs.Dind.Image)
	})

	t.Run("dind flag without docker layer fails before running", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		withTempToolHome(t)
		get := captureRunContainer(t)
		stubFlag(t, runToolCmd, "dind", "true")

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		require.ErrorContains(t, err, "--base docker")
		rs, _ := get()
		assert.Empty(t, rs.Image, "RunContainer should not be called")
	})

	t.Run("passes tool args", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		withTempToolHome(t)
		get := captureRunContainer(t)

		// Act
		err := runTool(runToolCmd, []string{"claude", "--dangerously-skip-permissions"})

		// Assert
		require.NoError(t, err)
		_, toolArgs := get()
		assert.Equal(t, []string{"--dangerously-skip-permissions"}, toolArgs)
	})
}

func Test_parseArgs(t *testing.T) {
	t.Run("tool name and image name", func(t *testing.T) {
		// Act
		result, err := parseArgs([]string{"claude"}, "agentic")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "claude", result.toolName)
		assert.Equal(t, "agentic-claude", result.imageName)
		assert.Empty(t, result.toolArgs)
		assert.False(t, result.skipEntrypoint)
	})

	t.Run("tool args", func(t *testing.T) {
		// Act
		result, err := parseArgs([]string{"claude", "--flag", "value"}, "agentic")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--flag", "value"}, result.toolArgs)
		assert.False(t, result.skipEntrypoint)
	})

	t.Run("dash dash sets skip entrypoint", func(t *testing.T) {
		// Act
		result, err := parseArgs([]string{"claude", "--", "bash", "-c", "echo hi"}, "agentic")

		// Assert
		require.NoError(t, err)
		assert.True(t, result.skipEntrypoint)
		assert.Equal(t, []string{"bash", "-c", "echo hi"}, result.toolArgs)
	})

	t.Run("dash dash no trailing args", func(t *testing.T) {
		// Act
		result, err := parseArgs([]string{"claude", "--"}, "agentic")

		// Assert
		require.NoError(t, err)
		assert.True(t, result.skipEntrypoint)
		assert.Empty(t, result.toolArgs)
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		_, err := parseArgs([]string{"bogus"}, "agentic")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bogus")
	})
}
