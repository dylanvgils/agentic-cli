package cmd

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDryRunBuild_printsDockerfile_skipsScript(t *testing.T) {
	// Arrange
	var scriptCalled bool
	stubBuildTool(t, func(_, _ string, _ tools.BuildOptions) error {
		scriptCalled = true
		return nil
	})

	// Act
	out := captureStdout(t, func() {
		err := dryRunBuild([]string{"claude"}, tools.BuildOptions{Versions: map[string]string{}})
		require.NoError(t, err)
	})

	// Assert
	assert.False(t, scriptCalled)
	assert.Contains(t, out, "FROM")
}

func TestBuildTools(t *testing.T) {
	t.Run("all tools when no args", func(t *testing.T) {
		// Arrange
		var built []string
		stubBuildTool(t, func(tool, _ string, _ tools.BuildOptions) error {
			built = append(built, tool)
			return nil
		})

		// Act
		err := buildTools([]string{}, "agentic", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"claude", "copilot", "opencode"}, built)
	})

	t.Run("single tool when arg given", func(t *testing.T) {
		// Arrange
		var built []string
		stubBuildTool(t, func(tool, _ string, _ tools.BuildOptions) error {
			built = append(built, tool)
			return nil
		})

		// Act
		out := captureStdout(t, func() {
			err := buildTools([]string{"claude"}, "agentic", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, []string{"claude"}, built)
		assert.Contains(t, out, "=> agentic-claude")
		assert.NotContains(t, out, "=> copilot")
		assert.NotContains(t, out, "=> opencode")
	})

	t.Run("base override shown", func(t *testing.T) {
		// Arrange
		stubBuildTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		opts := tools.BuildOptions{BaseOverride: "java", Versions: map[string]string{}}

		// Act
		out := captureStdout(t, func() {
			err := buildTools([]string{"claude"}, "agentic", opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "   base: java")
	})

	t.Run("base override with multiple extras shown", func(t *testing.T) {
		// Arrange
		stubBuildTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		opts := tools.BuildOptions{BaseOverride: "java,dotnet", Versions: map[string]string{}}

		// Act
		out := captureStdout(t, func() {
			err := buildTools([]string{"claude"}, "agentic", opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "   base: java, dotnet")
	})

	t.Run("base override hidden when empty", func(t *testing.T) {
		// Arrange
		stubBuildTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })

		// Act
		out := captureStdout(t, func() {
			err := buildTools([]string{"claude"}, "agentic", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.NotContains(t, out, "=> base:")
	})

	t.Run("script error propagates", func(t *testing.T) {
		// Arrange
		stubBuildTool(t, func(_, _ string, _ tools.BuildOptions) error {
			return fmt.Errorf("docker daemon not running")
		})

		// Act
		err := buildTools([]string{"claude"}, "agentic", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	t.Run("stops on first tool error", func(t *testing.T) {
		// Arrange
		var built []string
		stubBuildTool(t, func(tool, _ string, _ tools.BuildOptions) error {
			built = append(built, tool)
			return fmt.Errorf("fail on %s", tool)
		})

		// Act
		err := buildTools([]string{}, "agentic", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
		assert.Len(t, built, 1)
	})
}

func TestRunBuild(t *testing.T) {
	t.Run("no cache flag sets opt", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubBuildTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubPruneImages(t, func() (string, error) { return "", nil })

		require.NoError(t, buildCmd.Flags().Set("no-cache", "true"))
		defer buildCmd.Flags().Set("no-cache", "false") //nolint:errcheck

		// Act
		err := runBuild(buildCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, capturedOpts.NoCache)
	})

	t.Run("base flag sets opt", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		var capturedOpts tools.BuildOptions
		stubBuildTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubPruneImages(t, func() (string, error) { return "", nil })

		require.NoError(t, buildCmd.Flags().Set("base", "java"))
		defer buildCmd.Flags().Set("base", "") //nolint:errcheck

		// Act
		err := runBuild(buildCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "java", capturedOpts.BaseOverride)
	})

	t.Run("node flag sets opt", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubBuildTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubPruneImages(t, func() (string, error) { return "", nil })

		require.NoError(t, buildCmd.Flags().Set("node", "22"))
		defer buildCmd.Flags().Set("node", "") //nolint:errcheck

		// Act
		err := runBuild(buildCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "22", capturedOpts.Versions["node"])
	})

	t.Run("go flag sets opt", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubBuildTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubPruneImages(t, func() (string, error) { return "", nil })

		require.NoError(t, buildCmd.Flags().Set("go", "1.23"))
		defer buildCmd.Flags().Set("go", "") //nolint:errcheck

		// Act
		err := runBuild(buildCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "1.23", capturedOpts.Versions["go"])
	})

	t.Run("prune message shown when reclaimed non zero", func(t *testing.T) {
		// Arrange
		stubBuildTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		stubPruneImages(t, func() (string, error) { return "1.23GB", nil })

		// Act
		out := captureStdout(t, func() {
			err := runBuild(buildCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "=> pruned dangling images (reclaimed 1.23GB)")
	})

	t.Run("prune message hidden when reclaimed zero", func(t *testing.T) {
		// Arrange
		stubBuildTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		stubPruneImages(t, func() (string, error) { return "", nil })

		// Act
		out := captureStdout(t, func() {
			err := runBuild(buildCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.NotContains(t, out, "pruned dangling images")
	})
}
