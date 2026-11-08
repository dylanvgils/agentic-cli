package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
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
		out := iostreamstest.CaptureOut(t, func() {
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
		out := iostreamstest.CaptureOut(t, func() {
			err := runInspect(inspectCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "agentic-claude (a1b2c3d4e5f6)")
		assert.Contains(t, out, "work-claude")
	})
}

func Test_printImageDetail(t *testing.T) {
	t.Run("shows the namespace and tool, then the image detail", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, builtInfo, nil)

		// Act
		var err error
		out := iostreamstest.CaptureOut(t, func() { err = printImageDetail("claude", "agentic") })

		// Assert
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(out, "=> agentic/claude\n   image:"), out)
		assert.Contains(t, out, "agentic-claude (a1b2c3d4e5f6)")
	})

	t.Run("not built logs that no image was found", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)
		errLog := iostreamstest.StubErr(t)

		// Act
		var err error
		out := iostreamstest.CaptureOut(t, func() { err = printImageDetail("claude", "agentic") })

		// Assert
		require.NoError(t, err)
		assert.Empty(t, out)
		assert.Contains(t, errLog.String(), `no claude image found in namespace "agentic"`)
	})

	t.Run("docker error propagates", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, fmt.Errorf("docker daemon not running"))

		// Act
		err := printImageDetail("claude", "agentic")

		// Assert
		assert.ErrorContains(t, err, "docker daemon not running")
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, builtInfo, nil)

		// Act
		err := printImageDetail("bogus", "agentic")

		// Assert
		assert.ErrorContains(t, err, "bogus")
	})
}

func Test_printInfoDetail(t *testing.T) {
	for _, tc := range []struct {
		name string
		info *docker.ImageInfo
		want string
	}{
		{
			name: "every label set",
			info: &docker.ImageInfo{
				Image:          "agentic-claude",
				Tool:           "claude",
				ID:             "a1b2c3d4e5f6",
				Version:        "1.2.3",
				Base:           "node:24",
				Apt:            "jq,curl",
				CustomInstalls: "helm,golangci-lint",
				Built:          "2026-05-01",
				Pulled:         "2026-05-02T08:00:00Z",
				CLIVersion:     "v0.12.0",
				ProxyTrust:     true,
				Size:           "512MB",
			},
			want: "   image:     agentic-claude (a1b2c3d4e5f6)\n" +
				"   version:   1.2.3\n" +
				"   base:      node:24\n" +
				"   apt:       jq,curl\n" +
				"   installs:  helm,golangci-lint\n" +
				"   built:     2026-05-01\n" +
				"   pulled:    2026-05-02T08:00:00Z\n" +
				"   agentic:   v0.12.0\n" +
				"   proxy:     trusts the proxy CA\n" +
				"   size:      512MB\n",
		},
		{
			name: "empty labels show fallbacks and omit apt and installs",
			info: &docker.ImageInfo{Image: "agentic-claude", Tool: "claude", ID: "a1b2c3d4e5f6"},
			want: "   image:    agentic-claude (a1b2c3d4e5f6)\n" +
				"   version:  (unknown - rebuild to capture)\n" +
				"   base:     (unknown)\n" +
				"   built:    (unknown)\n" +
				"   pulled:   never\n" +
				"   agentic:  (unknown)\n" +
				"   proxy:    can't trust the proxy CA - rebuild with \"agentic update claude\"\n" +
				"   size:     (unknown)\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			var err error
			out := iostreamstest.CaptureOut(t, func() { err = printInfoDetail(tc.info) })

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, out)
		})
	}
}

func Test_printAllNamespaceDetail(t *testing.T) {
	t.Run("shows detail for every image of the tool", func(t *testing.T) {
		// Arrange
		var capturedFilters []docker.ImageFilter
		workInfo := &docker.ImageInfo{Image: "work-claude", Namespace: "work", Tool: "claude"}
		stubListAllImages(t, func(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			capturedFilters = filters
			return []*docker.ImageInfo{builtInfo, workInfo}, nil
		})

		// Act
		var err error
		out := iostreamstest.CaptureOut(t, func() { err = printAllNamespaceDetail("claude") })

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []docker.ImageFilter{docker.ToolFilter("claude")}, capturedFilters)
		assert.Contains(t, out, "agentic-claude")
		assert.Contains(t, out, "work-claude")
	})

	t.Run("no match prints message", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, nil
		})
		errLog := iostreamstest.StubErr(t)

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
}
