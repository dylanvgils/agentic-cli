package cli

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runInspect(t *testing.T) {
	// Keep the repo's own .agenticrc.toml out of the test
	t.Chdir(t.TempDir())

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		err := runInspect(inspectCmd, []string{"bogus"})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bogus")
	})

	t.Run("shows that tool's image in the namespace", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, builtInfo, nil)

		// Act
		out := captureLog(t, func() {
			err := runInspect(inspectCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "agentic-claude (a1b2c3d4e5f6)")
	})

	t.Run("with --all shows the tool in every namespace", func(t *testing.T) {
		// Arrange
		workInfo := &docker.ImageInfo{Image: "work-claude", Namespace: "work", Tool: "claude"}
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{builtInfo, workInfo}, nil
		})
		stubFlag(t, inspectCmd, "all", "true")

		// Act
		out := captureLog(t, func() {
			err := runInspect(inspectCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "agentic-claude (a1b2c3d4e5f6)")
		assert.Contains(t, out, "work-claude")
	})
}

func Test_printImageDetail(t *testing.T) {
	t.Run("shows detail for active namespace", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, builtInfo, nil)

		// Act
		out := captureLog(t, func() {
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

	t.Run("not built logs that no image was found", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)
		errLog := stubErrLog(t)

		// Act
		out := captureLog(t, func() {
			err := printImageDetail("claude", "agentic")
			require.NoError(t, err)
		})

		// Assert
		assert.Empty(t, out)
		assert.Contains(t, errLog.String(), `no claude image found in namespace "agentic"`)
	})

	t.Run("shows custom installs when present", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{
			Image: "agentic-claude", ID: "a1b2c3d4e5f6", CustomInstalls: "helm,golangci-lint",
		}, nil)

		// Act
		out := captureLog(t, func() {
			err := printImageDetail("claude", "agentic")
			require.NoError(t, err)
		})

		// Assert
		assert.Regexp(t, `installs:\s+helm,golangci-lint`, out)
	})

	t.Run("omits installs line when empty", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, builtInfo, nil)

		// Act
		out := captureLog(t, func() {
			err := printImageDetail("claude", "agentic")
			require.NoError(t, err)
		})

		// Assert
		assert.NotContains(t, out, "installs:")
	})

	t.Run("empty labels shows fallbacks", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{
			Image: "agentic-claude",
			ID:    "a1b2c3d4e5f6",
		}, nil)

		// Act
		out := captureLog(t, func() {
			err := printImageDetail("claude", "agentic")
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "(unknown - rebuild to capture)")
		assert.Contains(t, out, "(unknown)")
		assert.Regexp(t, `pulled:\s+never`, out)
	})

	t.Run("shows build provenance and proxy trust", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{
			Image:      "agentic-claude",
			ID:         "a1b2c3d4e5f6",
			Pulled:     "2026-05-02T08:00:00Z",
			CLIVersion: "v0.12.0",
			ProxyTrust: true,
		}, nil)

		// Act
		out := captureLog(t, func() {
			err := printImageDetail("claude", "agentic")
			require.NoError(t, err)
		})

		// Assert
		assert.Regexp(t, `pulled:\s+2026-05-02T08:00:00Z`, out)
		assert.Regexp(t, `agentic:\s+v0\.12\.0`, out)
		assert.Regexp(t, `proxy:\s+trusts the proxy CA`, out)
	})

	t.Run("image without proxy trust says to rebuild", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{
			Image: "agentic-claude",
			Tool:  "claude",
			ID:    "a1b2c3d4e5f6",
		}, nil)

		// Act
		out := captureLog(t, func() {
			err := printImageDetail("claude", "agentic")
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, `can't trust the proxy CA - rebuild with "agentic update claude"`)
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

	t.Run("shows apt packages when present", func(t *testing.T) {
		// Arrange
		info := *builtInfo
		info.Apt = "jq,curl"
		stubInspectImage(t, &info, nil)

		// Act
		out := captureLog(t, func() {
			err := printImageDetail("claude", "agentic")
			require.NoError(t, err)
		})

		// Assert
		assert.Regexp(t, `apt:\s+jq,curl`, out)
	})
}

func Test_printAllNamespaceDetail(t *testing.T) {
	t.Run("shows detail for all matching images", func(t *testing.T) {
		// Arrange
		workInfo := &docker.ImageInfo{
			Image: "work-claude", Namespace: "work", Tool: "claude",
			ID: "deadbeef1234", Version: "2.0", Base: "node@24", Built: "2026-05-02", Size: "600MB",
		}
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{builtInfo, workInfo}, nil
		})

		// Act
		out := captureLog(t, func() {
			err := printAllNamespaceDetail("claude")
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "agentic-claude")
		assert.Contains(t, out, "work-claude")
	})

	t.Run("no match prints message", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, nil
		})
		errLog := stubErrLog(t)

		// Act
		err := printAllNamespaceDetail("unknown")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, errLog.String(), `no images found for tool "unknown"`)
	})

	t.Run("docker error propagates", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		})

		// Act
		err := printAllNamespaceDetail("claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	t.Run("tool filter passed to listAllImages", func(t *testing.T) {
		// Arrange
		var capturedFilters []docker.ImageFilter
		stubListAllImages(t, func(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			capturedFilters = filters
			return []*docker.ImageInfo{builtInfo}, nil
		})

		// Act
		err := printAllNamespaceDetail("claude")
		require.NoError(t, err)

		// Assert
		assert.Equal(t, []docker.ImageFilter{docker.ToolFilter("claude")}, capturedFilters)
	})
}
