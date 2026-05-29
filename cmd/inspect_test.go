package cmd

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var builtInfo = &docker.ImageInfo{
	Image:   "agentic-claude",
	ID:      "a1b2c3d4e5f6",
	Version: "1.2.3",
	Base:    "node:24",
	Built:   "2026-05-01",
	Size:    "512MB",
}

func TestRunInspect(t *testing.T) {
	t.Run("all tools when no args", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, builtInfo, nil)

		// Act
		out := captureStdout(t, func() {
			err := runInspect(inspectCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "=> claude")
		assert.Contains(t, out, "=> copilot")
		assert.Contains(t, out, "=> opencode")
	})

	t.Run("single tool when arg given", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, builtInfo, nil)

		// Act
		out := captureStdout(t, func() {
			err := runInspect(inspectCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "=> claude")
		assert.NotContains(t, out, "=> copilot")
		assert.NotContains(t, out, "=> opencode")
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, builtInfo, nil)

		// Act
		err := runInspect(inspectCmd, []string{"bogus"})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bogus")
	})

	t.Run("built image prints all fields", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, builtInfo, nil)

		// Act
		out := captureStdout(t, func() {
			err := runInspect(inspectCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "agentic-claude (a1b2c3d4e5f6)")
		assert.Contains(t, out, "1.2.3")
		assert.Contains(t, out, "node:24")
		assert.Contains(t, out, "2026-05-01")
		assert.Contains(t, out, "512MB")
	})

	t.Run("not built prints fallback", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)

		// Act
		out := captureStdout(t, func() {
			err := runInspect(inspectCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "agentic-claude (not built)")
	})

	t.Run("empty labels prints fallbacks", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{
			Image: "agentic-claude",
			ID:    "a1b2c3d4e5f6",
		}, nil)

		// Act
		out := captureStdout(t, func() {
			err := runInspect(inspectCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "(unknown - rebuild to capture)")
		assert.Contains(t, out, "(unknown)")
	})

	t.Run("docker error propagates", func(t *testing.T) {
		// Arrange
		orig := inspectImage
		inspectImage = func(_ string) (*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		}
		defer func() { inspectImage = orig }()

		// Act
		err := runInspect(inspectCmd, []string{"claude"})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	t.Run("invalid output format returns error", func(t *testing.T) {
		// Arrange
		outputFmt = "json"
		defer func() { outputFmt = "default" }()

		// Act
		err := runInspect(inspectCmd, []string{"claude"})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "json")
	})

	t.Run("table output shows header", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, builtInfo, nil)
		outputFmt = "table"
		defer func() { outputFmt = "default" }()

		// Act
		out := captureStdout(t, func() {
			err := runInspect(inspectCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "TOOL")
		assert.Contains(t, out, "IMAGE")
		assert.Contains(t, out, "VERSION")
		assert.Contains(t, out, "BUILT")
		assert.Contains(t, out, "SIZE")
	})

	t.Run("table output built image shows fields", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, builtInfo, nil)
		outputFmt = "table"
		defer func() { outputFmt = "default" }()

		// Act
		out := captureStdout(t, func() {
			err := runInspect(inspectCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "claude")
		assert.Contains(t, out, "1.2.3")
		assert.Contains(t, out, "2026-05-01")
		assert.Contains(t, out, "512MB")
	})

	t.Run("table output not built shows dashes", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)
		outputFmt = "table"
		defer func() { outputFmt = "default" }()

		// Act
		out := captureStdout(t, func() {
			err := runInspect(inspectCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "(not built)")
	})

	t.Run("table output empty labels shows unknown", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{Image: "agentic-claude", ID: "abc"}, nil)
		outputFmt = "table"
		defer func() { outputFmt = "default" }()

		// Act
		out := captureStdout(t, func() {
			err := runInspect(inspectCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "(unknown)")
	})

	t.Run("table output docker error propagates", func(t *testing.T) {
		// Arrange
		orig := inspectImage
		inspectImage = func(_ string) (*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		}
		defer func() { inspectImage = orig }()
		outputFmt = "table"
		defer func() { outputFmt = "default" }()

		// Act
		err := runInspect(inspectCmd, []string{"claude"})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})
}
