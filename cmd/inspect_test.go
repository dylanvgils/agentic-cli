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
	Prefix:  "agentic",
	Tool:    "claude",
	ID:      "a1b2c3d4e5f6",
	Version: "1.2.3",
	Base:    "node:24",
	Built:   "2026-05-01",
	Size:    "512MB",
}

func TestRunInspect(t *testing.T) {
	t.Run("no args shows table of all images", func(t *testing.T) {
		// Arrange
		stubListAllAgenticImages(t, func() ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{builtInfo}, nil
		})

		// Act
		out := captureStdout(t, func() {
			err := runInspect(inspectCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "PREFIX")
		assert.Contains(t, out, "TOOL")
		assert.Contains(t, out, "VERSION")
		assert.Contains(t, out, "BASE")
		assert.Contains(t, out, "BUILT")
		assert.Contains(t, out, "SIZE")
		assert.Contains(t, out, "agentic")
		assert.Contains(t, out, "claude")
	})

	t.Run("no args empty shows placeholder", func(t *testing.T) {
		// Arrange
		stubListAllAgenticImages(t, func() ([]*docker.ImageInfo, error) {
			return nil, nil
		})

		// Act
		out := captureStdout(t, func() {
			err := runInspect(inspectCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "no agentic images found")
	})

	t.Run("table truncates long base field", func(t *testing.T) {
		// Arrange
		longBase := "node@24,java@21,dotnet@9,go@1.26.3,extra@1,another@2"
		stubListAllAgenticImages(t, func() ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{{
				Image: "agentic-claude", Prefix: "agentic", Tool: "claude",
				Version: "1.0", Base: longBase, Built: "2026-05-01", Size: "1GB",
			}}, nil
		})

		// Act
		out := captureStdout(t, func() {
			err := runInspect(inspectCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "...")
		assert.NotContains(t, out, longBase)
	})

	t.Run("no args docker error propagates", func(t *testing.T) {
		// Arrange
		stubListAllAgenticImages(t, func() ([]*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		})

		// Act
		err := runInspect(inspectCmd, []string{})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	t.Run("tool arg shows detail for active prefix", func(t *testing.T) {
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

	t.Run("tool arg not built shows fallback", func(t *testing.T) {
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

	t.Run("tool arg empty labels shows fallbacks", func(t *testing.T) {
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

	t.Run("tool arg docker error propagates", func(t *testing.T) {
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

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, builtInfo, nil)

		// Act
		err := runInspect(inspectCmd, []string{"bogus"})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bogus")
	})

	t.Run("all flag shows detail for all prefixes of tool", func(t *testing.T) {
		// Arrange
		workInfo := &docker.ImageInfo{
			Image: "work-claude", Prefix: "work", Tool: "claude",
			ID: "deadbeef1234", Version: "2.0", Base: "node@24", Built: "2026-05-02", Size: "600MB",
		}
		stubListAllAgenticImages(t, func() ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{builtInfo, workInfo}, nil
		})

		require.NoError(t, inspectCmd.Flags().Set("all", "true"))
		defer inspectCmd.Flags().Set("all", "false") //nolint:errcheck

		// Act
		out := captureStdout(t, func() {
			err := runInspect(inspectCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "agentic-claude")
		assert.Contains(t, out, "work-claude")
	})
}

func TestTruncate(t *testing.T) {
	t.Run("short string unchanged", func(t *testing.T) {
		// Act
		result := truncate("node@24", baseMaxLen)

		// Assert
		assert.Equal(t, "node@24", result)
	})

	t.Run("long string truncated with ellipsis", func(t *testing.T) {
		// Arrange
		long := "node@24,java@21,dotnet@9,go@1.26,extra@1,another@2,more@3"

		// Act
		result := truncate(long, baseMaxLen)

		// Assert
		assert.Len(t, result, baseMaxLen+3)
		assert.True(t, len(result) <= baseMaxLen+3)
		assert.Contains(t, result, "...")
	})
}
