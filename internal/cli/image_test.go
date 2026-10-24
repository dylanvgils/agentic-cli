package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var builtInfo = &docker.ImageInfo{
	Image:     "agentic-claude",
	Namespace: "agentic",
	Tool:      "claude",
	ID:        "a1b2c3d4e5f6",
	Version:   "1.2.3",
	Base:      "node:24",
	Built:     "2026-05-01",
	Size:      "512MB",
}

func Test_runImageList(t *testing.T) {
	// Keep the repo's own .agenticrc.toml out of the test
	t.Chdir(t.TempDir())

	t.Run("without --all shows the namespace's images", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{builtInfo}, nil
		})

		// Act
		out := captureLog(t, func() {
			err := runImageList(imageListCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Regexp(t, `claude\s+agentic\s+1\.2\.3`, out)
	})

	t.Run("with --all shows all namespaces", func(t *testing.T) {
		// Arrange
		workInfo := &docker.ImageInfo{Image: "work-claude", Namespace: "work", Tool: "claude"}
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{builtInfo, workInfo}, nil
		})
		stubFlag(t, imageListCmd, "all", "true")

		// Act
		out := captureLog(t, func() {
			err := runImageList(imageListCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Regexp(t, `claude\s+agentic`, out)
		assert.Regexp(t, `claude\s+work`, out)
	})

	t.Run("images shortcut takes --all", func(t *testing.T) {
		// Arrange
		workInfo := &docker.ImageInfo{Image: "work-claude", Namespace: "work", Tool: "claude"}
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{builtInfo, workInfo}, nil
		})
		stubFlag(t, imagesCmd, "all", "true")

		// Act
		out := captureLog(t, func() {
			err := runImageList(imagesCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Regexp(t, `claude\s+work`, out)
	})
}

func Test_runImageTable(t *testing.T) {
	t.Run("with namespace shows namespace images only", func(t *testing.T) {
		// Arrange - stub returns only the agentic image (as Docker would after label filtering)
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{builtInfo}, nil
		})

		// Act
		out := captureLog(t, func() {
			err := runImageTable("agentic")
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "claude")
		assert.NotContains(t, out, "work")
	})

	t.Run("results are sorted by tool name", func(t *testing.T) {
		// Arrange
		opencode := &docker.ImageInfo{Image: "agentic-opencode", Namespace: "agentic", Tool: "opencode", Version: "0.1.0"}
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{opencode, builtInfo}, nil
		})

		// Act
		out := captureLog(t, func() {
			err := runImageTable("")
			require.NoError(t, err)
		})

		// Assert
		assert.Less(t, strings.Index(out, "claude"), strings.Index(out, "opencode"))
	})

	t.Run("docker error propagates", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		})

		// Act
		err := runImageTable("")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})
}

func Test_writeImageTable(t *testing.T) {
	t.Run("shows each image with its namespace", func(t *testing.T) {
		// Act
		out := captureLog(t, func() {
			err := writeImageTable("agentic", []*docker.ImageInfo{builtInfo})
			require.NoError(t, err)
		})

		// Assert
		assert.Regexp(t, `TOOL\s+NAMESPACE\s+VERSION\s+BASE\s+BUILT\s+SIZE`, out)
		assert.Regexp(t, `claude\s+agentic\s+1\.2\.3\s+node:24\s+2026-05-01\s+512MB`, out)
	})

	t.Run("empty names the namespace", func(t *testing.T) {
		// Arrange
		logs := stubErrLog(t)

		// Act
		err := writeImageTable("agentic", nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logs.String(), "no images found in namespace \"agentic\"")
	})

	t.Run("empty in every namespace says so", func(t *testing.T) {
		// Arrange
		logs := stubErrLog(t)

		// Act
		err := writeImageTable("", nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logs.String(), "no images found\n")
	})

	t.Run("truncates long base field", func(t *testing.T) {
		// Arrange
		longBase := "node@24,java@21,dotnet@9,go@1.26.3,extra@1,another@2"

		// Act
		out := captureLog(t, func() {
			err := writeImageTable("agentic", []*docker.ImageInfo{{
				Image: "agentic-claude", Namespace: "agentic", Tool: "claude",
				Version: "1.0", Base: longBase, Built: "2026-05-01", Size: "1GB",
			}})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "...")
		assert.NotContains(t, out, longBase)
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

func Test_runImagePrune(t *testing.T) {
	// Arrange
	logBuf := stubErrLog(t)
	pruned := false
	stubDocker(t, &fakeDocker{pruneImages: func() (string, error) {
		pruned = true
		return "", nil
	}})

	// Act
	err := runImagePrune(imagePruneCmd, nil)

	// Assert
	require.NoError(t, err)
	assert.True(t, pruned)
	assert.Contains(t, logBuf.String(), "agentic: removing dangling images")
}
