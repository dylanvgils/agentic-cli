package cmd

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDryRunUpdate(t *testing.T) {
	t.Run("prints dockerfile skips script", func(t *testing.T) {
		// Arrange
		var scriptCalled bool
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error {
			scriptCalled = true
			return nil
		})

		// Act
		out := captureStdout(t, func() {
			err := dryRunUpdate([]string{"claude"}, tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.False(t, scriptCalled)
		assert.Contains(t, out, "FROM")
	})

	t.Run("without tool arg returns error", func(t *testing.T) {
		// Act
		err := dryRunUpdate([]string{}, tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--dry-run requires a tool argument")
	})

	t.Run("recovers base from image label", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{Base: "node@24.0.0,java@21.0.1"}, nil)

		// Act
		out := captureStdout(t, func() {
			err := dryRunUpdate([]string{"claude"}, tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "temurin")
	})

	t.Run("explicit base flag takes precedence", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{Base: "node@24.0.0,go@1.22"}, nil)
		opts := tools.BuildOptions{BaseOverride: "java", Versions: map[string]string{}}

		// Act
		out := captureStdout(t, func() {
			err := dryRunUpdate([]string{"claude"}, opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "temurin")
		assert.NotContains(t, out, "go.dev")
	})
}

func TestUpdateTools(t *testing.T) {
	t.Run("all built updates all", func(t *testing.T) {
		// Arrange
		var updated []string
		stubUpdateTool(t, func(tool, _ string, _ tools.BuildOptions) error {
			updated = append(updated, tool)
			return nil
		})
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)

		// Act
		err := updateTools([]string{}, tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"claude", "copilot", "opencode"}, updated)
	})

	t.Run("all unbuilt skips all prints message", func(t *testing.T) {
		// Arrange
		var updated []string
		stubUpdateTool(t, func(tool, _ string, _ tools.BuildOptions) error {
			updated = append(updated, tool)
			return nil
		})
		stubInspectImage(t, nil, nil)

		// Act
		out := captureStdout(t, func() {
			err := updateTools([]string{}, tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.Empty(t, updated)
		assert.Contains(t, out, "No tools are built.")
	})

	t.Run("mixed built skips unbuilt", func(t *testing.T) {
		// Arrange
		var updated []string
		stubUpdateTool(t, func(tool, _ string, _ tools.BuildOptions) error {
			updated = append(updated, tool)
			return nil
		})

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
	})

	t.Run("single tool always updates", func(t *testing.T) {
		// Arrange
		var updated []string
		stubUpdateTool(t, func(tool, _ string, _ tools.BuildOptions) error {
			updated = append(updated, tool)
			return nil
		})
		stubInspectImage(t, nil, nil)

		// Act
		err := updateTools([]string{"claude"}, tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"claude"}, updated)
	})

	t.Run("stops on first tool error", func(t *testing.T) {
		// Arrange
		var updated []string
		stubUpdateTool(t, func(tool, _ string, _ tools.BuildOptions) error {
			updated = append(updated, tool)
			return fmt.Errorf("fail on %s", tool)
		})
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)

		// Act
		err := updateTools([]string{}, tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
		assert.Len(t, updated, 1)
	})
}

func TestUpdateOneTool(t *testing.T) {
	t.Run("version changed reported", func(t *testing.T) {
		// Arrange
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })

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
	})

	t.Run("version up to date reported", func(t *testing.T) {
		// Arrange
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)

		// Act
		out := captureStdout(t, func() {
			err := updateOneTool("claude", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "=> version: 1.0.0 (up to date)")
	})

	t.Run("script error propagates", func(t *testing.T) {
		// Arrange
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error {
			return fmt.Errorf("docker daemon not running")
		})
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)

		// Act
		err := updateOneTool("claude", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})
}

func TestRunUpdate(t *testing.T) {
	t.Run("no cache flag sets opt", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() (string, error) { return "", nil })

		require.NoError(t, updateCmd.Flags().Set("no-cache", "true"))
		defer updateCmd.Flags().Set("no-cache", "false") //nolint:errcheck

		// Act
		err := runUpdate(updateCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, capturedOpts.NoCache)
	})

	t.Run("prune message shown when reclaimed non zero", func(t *testing.T) {
		// Arrange
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() (string, error) { return "512MB", nil })

		// Act
		out := captureStdout(t, func() {
			err := runUpdate(updateCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "=> pruned dangling images (reclaimed 512MB)")
	})
}
