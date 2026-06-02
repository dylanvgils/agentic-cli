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

func Test_runInspect(t *testing.T) {
	t.Run("no args propagates table error", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, fmt.Errorf("table error")
		})

		// Act
		err := runInspect(inspectCmd, []string{})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table error")
	})

	t.Run("tool arg propagates detail error", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, fmt.Errorf("detail error"))

		// Act
		err := runInspect(inspectCmd, []string{"claude"})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "detail error")
	})

	t.Run("all flag propagates all-prefix error", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, fmt.Errorf("all error")
		})
		require.NoError(t, inspectCmd.Flags().Set("all", "true"))
		defer inspectCmd.Flags().Set("all", "false") //nolint:errcheck

		// Act
		err := runInspect(inspectCmd, []string{"claude"})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "all error")
	})
}

func Test_runInspectTable(t *testing.T) {
	t.Run("shows headers and image data", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{builtInfo}, nil
		})

		// Act
		out := captureStdout(t, func() {
			err := runInspectTable()
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

	t.Run("empty shows placeholder", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, nil
		})

		// Act
		out := captureStdout(t, func() {
			err := runInspectTable()
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "no agentic images found")
	})

	t.Run("truncates long base field", func(t *testing.T) {
		// Arrange
		longBase := "node@24,java@21,dotnet@9,go@1.26.3,extra@1,another@2"
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{{
				Image: "agentic-claude", Prefix: "agentic", Tool: "claude",
				Version: "1.0", Base: longBase, Built: "2026-05-01", Size: "1GB",
			}}, nil
		})

		// Act
		out := captureStdout(t, func() {
			err := runInspectTable()
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "...")
		assert.NotContains(t, out, longBase)
	})

	t.Run("docker error propagates", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		})

		// Act
		err := runInspectTable()

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})
}

func Test_printImageDetail(t *testing.T) {
	t.Run("shows detail for active prefix", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, builtInfo, nil)

		// Act
		out := captureStdout(t, func() {
			err := printImageDetail("claude", "agentic")
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "agentic-claude (a1b2c3d4e5f6)")
		assert.Contains(t, out, "1.2.3")
		assert.Contains(t, out, "node:24")
		assert.Contains(t, out, "2026-05-01")
		assert.Contains(t, out, "512MB")
	})

	t.Run("not built shows fallback", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)

		// Act
		out := captureStdout(t, func() {
			err := printImageDetail("claude", "agentic")
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "agentic-claude (not built)")
	})

	t.Run("empty labels shows fallbacks", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{
			Image: "agentic-claude",
			ID:    "a1b2c3d4e5f6",
		}, nil)

		// Act
		out := captureStdout(t, func() {
			err := printImageDetail("claude", "agentic")
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "(unknown - rebuild to capture)")
		assert.Contains(t, out, "(unknown)")
	})

	t.Run("docker error propagates", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, fmt.Errorf("docker daemon not running"))

		// Act
		err := printImageDetail("claude", "agentic")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, builtInfo, nil)

		// Act
		err := printImageDetail("bogus", "agentic")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bogus")
	})
}

func Test_printAllPrefixDetail(t *testing.T) {
	t.Run("shows detail for all matching images", func(t *testing.T) {
		// Arrange
		workInfo := &docker.ImageInfo{
			Image: "work-claude", Prefix: "work", Tool: "claude",
			ID: "deadbeef1234", Version: "2.0", Base: "node@24", Built: "2026-05-02", Size: "600MB",
		}
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{builtInfo, workInfo}, nil
		})

		// Act
		out := captureStdout(t, func() {
			err := printAllPrefixDetail("claude")
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "agentic-claude")
		assert.Contains(t, out, "work-claude")
	})

	t.Run("no match prints message", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{builtInfo}, nil
		})

		// Act
		out := captureStdout(t, func() {
			err := printAllPrefixDetail("unknown")
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, `no images found for tool "unknown"`)
	})

	t.Run("docker error propagates", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		})

		// Act
		err := printAllPrefixDetail("claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})
}

func Test_truncate(t *testing.T) {
	t.Run("short string unchanged", func(t *testing.T) {
		// Act
		result := truncate("node@24", baseMaxLength)

		// Assert
		assert.Equal(t, "node@24", result)
	})

	t.Run("long string truncated with ellipsis", func(t *testing.T) {
		// Arrange
		long := "node@24,java@21,dotnet@9,go@1.26,extra@1,another@2,more@3"

		// Act
		result := truncate(long, baseMaxLength)

		// Assert
		assert.Len(t, result, baseMaxLength+3)
		assert.True(t, len(result) <= baseMaxLength+3)
		assert.Contains(t, result, "...")
	})
}
