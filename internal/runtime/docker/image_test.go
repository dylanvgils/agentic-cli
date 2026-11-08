package docker

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/infra/buildinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fullImageJSON = `{
	"Id": "sha256:a1b2c3d4e5f6abcdef012345678901234567890",
	"Config": {
		"User": "claude",
		"Labels": {
			"agentic.tool": "claude",
			"agentic.namespace": "agentic",
			"agentic.tool.version": "1.2.3",
			"agentic.base": "node:24",
			"agentic.built": "2026-05-01",
			"agentic.version": "v1.0.0",
			"agentic.custom-installs": "helm,golangci-lint",
			"agentic.custom-installs.hash": "abc123",
			"agentic.proxy-trust": "true",
			"agentic.dockerd.parent": "sha256:0f0f0f0f0f0f"
		}
	}
}`

func TestInspectImage(t *testing.T) {
	client := newTestClient()

	t.Run("inspects the image and its size into every field", func(t *testing.T) {
		// Arrange
		callNum := 0
		var inspectArgs, lsArgs []string
		stubDockerRun(t, client, func(args ...string) (string, error) {
			callNum++
			switch callNum {
			case 1:
				inspectArgs = args
				return fullImageJSON, nil
			case 2:
				lsArgs = args
				return "1.23GB", nil
			}
			return "", nil
		})

		// Act
		info, err := client.InspectImage("agentic-claude")

		// Assert
		require.NoError(t, err)
		require.NotNil(t, info)
		assert.Contains(t, inspectArgs, "agentic-claude")
		assert.Contains(t, lsArgs, "--filter=reference=agentic-claude")
		assert.Equal(t, "agentic-claude", info.Image)
		assert.Equal(t, "agentic", info.Namespace)
		assert.Equal(t, "claude", info.Tool)
		assert.Equal(t, "a1b2c3d4e5f6", info.ID)
		assert.Equal(t, "sha256:a1b2c3d4e5f6abcdef012345678901234567890", info.FullID)
		assert.Equal(t, "claude", info.User)
		assert.Equal(t, "1.2.3", info.Version)
		assert.Equal(t, "node:24", info.Base)
		assert.Equal(t, "2026-05-01", info.Built)
		assert.Equal(t, "v1.0.0", info.CLIVersion)
		assert.Equal(t, "1.23GB", info.Size)
		assert.Equal(t, "helm,golangci-lint", info.CustomInstalls)
		assert.Equal(t, "abc123", info.CustomInstallsHash)
		assert.True(t, info.ProxyTrust)
		assert.Equal(t, "sha256:0f0f0f0f0f0f", info.DockerdParent)
	})

	t.Run("no labels", func(t *testing.T) {
		// Arrange
		callNum := 0
		stubDockerRun(t, client, func(args ...string) (string, error) {
			callNum++
			if callNum == 1 {
				return `{"Id":"sha256:a1b2c3d4e5f6abcdef012345","Config":{"Labels":{}}}`, nil
			}
			return "", nil
		})

		// Act
		info, err := client.InspectImage("agentic-claude")

		// Assert
		require.NoError(t, err)
		require.NotNil(t, info)
		assert.Empty(t, info.Version)
		assert.Empty(t, info.Base)
		assert.Empty(t, info.Built)
		assert.Empty(t, info.Size)
		assert.Empty(t, info.CustomInstalls)
	})

	t.Run("docker error returns nil", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", fmt.Errorf("No such image: agentic-missing"))

		// Act
		info, err := client.InspectImage("agentic-missing")

		// Assert
		require.NoError(t, err)
		assert.Nil(t, info)
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "not json", nil)

		// Act
		info, err := client.InspectImage("agentic-claude")

		// Assert
		require.Error(t, err)
		assert.Nil(t, info)
	})

}

func TestSaveImage(t *testing.T) {
	client := newTestClient()

	t.Run("streams docker save to the writer", func(t *testing.T) {
		// Arrange
		var gotArgs []string
		fakeOf(client).stream = func(w io.Writer, args ...string) error {
			gotArgs = args
			_, err := io.WriteString(w, "tar-bytes")
			return err
		}
		t.Cleanup(func() { fakeOf(client).stream = nil })
		var out strings.Builder

		// Act
		err := client.SaveImage("agentic-claude", &out)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"save", "agentic-claude"}, gotArgs)
		assert.Equal(t, "tar-bytes", out.String())
	})

	t.Run("wraps the error", func(t *testing.T) {
		// Arrange
		fakeOf(client).stream = func(io.Writer, ...string) error { return errors.New("boom") }
		t.Cleanup(func() { fakeOf(client).stream = nil })

		// Act
		err := client.SaveImage("agentic-claude", io.Discard)

		// Assert
		assert.ErrorContains(t, err, "save image agentic-claude: boom")
	})
}

func Test_extractShortID(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   string
		want string
	}{
		{name: "returns 12-char short ID", id: "sha256:a1b2c3d4e5f6abcdef012345678901234567890", want: "a1b2c3d4e5f6"},
		{name: "ID shorter than 19 chars returns empty", id: "sha256:short"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			id := extractShortID(tc.id)

			// Assert
			assert.Equal(t, tc.want, id)
		})
	}
}

func Test_resolveToolName(t *testing.T) {
	for _, tc := range []struct {
		name          string
		image         string
		toolLabel     string
		nsLabel       string
		wantNamespace string
		wantTool      string
	}{
		{name: "labels take precedence over the parsed name", image: "agentic-claude", toolLabel: "copilot", nsLabel: "myproject", wantNamespace: "myproject", wantTool: "copilot"},
		{name: "falls back to the parsed name without labels", image: "myproject-claude", wantNamespace: "myproject", wantTool: "claude"},
		{name: "unknown tool with no label returns empty tool", image: "agentic-bogus"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			namespace, tool := resolveToolName(tc.image, tc.toolLabel, tc.nsLabel)

			// Assert
			assert.Equal(t, tc.wantNamespace, namespace)
			assert.Equal(t, tc.wantTool, tool)
		})
	}
}

func Test_listAllRepositories(t *testing.T) {
	client := newTestClient()

	t.Run("lists the project's repositories with the extra filters, once each", func(t *testing.T) {
		// Arrange
		var capturedArgs []string
		stubDockerRun(t, client, func(args ...string) (string, error) {
			capturedArgs = args
			return "agentic-claude\n<none>\nagentic-claude\nmyproject-copilot\n", nil
		})

		// Act
		repos, err := client.listAllRepositories(ToolFilter("claude"))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"agentic-claude", "myproject-copilot"}, repos)
		assert.Contains(t, capturedArgs, "--filter=label=project=agentic-cli")
		assert.Contains(t, capturedArgs, "--filter=label=agentic.tool=claude")
	})

	t.Run("docker error propagates", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", fmt.Errorf("docker daemon not running"))

		// Act
		_, err := client.listAllRepositories()

		// Assert
		require.Error(t, err)
	})

}

func TestListAllImages(t *testing.T) {
	client := newTestClient()

	t.Run("returns parsed images", func(t *testing.T) {
		// Arrange
		myprojectJSON := `{"Id":"sha256:b2c3d4e5f6a7bcdef012345678901234567890","Config":{"Labels":{"agentic.tool":"claude","agentic.namespace":"myproject"}}}`
		callNum := 0
		stubDockerRun(t, client, func(args ...string) (string, error) {
			callNum++
			switch callNum {
			case 1: // images --filter label=project=agentic-cli
				return "agentic-claude\nmyproject-claude\n", nil
			case 2: // inspect agentic-claude
				return fullImageJSON, nil
			case 3: // image ls size agentic-claude
				return "512MB", nil
			case 4: // inspect myproject-claude
				return myprojectJSON, nil
			case 5: // image ls size myproject-claude
				return "512MB", nil
			}
			return "", nil
		})

		// Act
		images, err := client.ListAllImages()

		// Assert
		require.NoError(t, err)
		require.Len(t, images, 2)
		assert.Equal(t, "agentic", images[0].Namespace)
		assert.Equal(t, "claude", images[0].Tool)
		assert.Equal(t, "myproject", images[1].Namespace)
		assert.Equal(t, "claude", images[1].Tool)
	})

	t.Run("skips images where inspect returns nil", func(t *testing.T) {
		// Arrange
		callNum := 0
		stubDockerRun(t, client, func(args ...string) (string, error) {
			callNum++
			switch callNum {
			case 1: // images --filter
				return "agentic-claude\nagentic-copilot\n", nil
			case 2: // inspect claude - image does not exist
				return "", fmt.Errorf("No such image: agentic-claude")
			case 3: // inspect copilot
				return `{"Id":"sha256:b2c3d4e5f6a7bcdef012345678901234567890","Config":{"Labels":{"agentic.tool":"copilot"}}}`, nil
			case 4: // image ls size copilot
				return "512MB", nil
			}
			return "", nil
		})

		// Act
		images, err := client.ListAllImages()

		// Assert
		require.NoError(t, err)
		require.Len(t, images, 1)
		assert.Equal(t, "copilot", images[0].Tool)
	})
}

func TestImageSize(t *testing.T) {
	client := newTestClient()

	t.Run("returns size", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "1.23GB", nil)

		// Act
		size := client.imageSize("agentic-claude")

		// Assert
		assert.Equal(t, "1.23GB", size)
	})

	t.Run("error returns empty", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", fmt.Errorf("docker error"))

		// Act
		size := client.imageSize("agentic-claude")

		// Assert
		assert.Empty(t, size)
	})
}

func TestImageInfoBuiltBefore(t *testing.T) {
	cutoff := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name  string
		built string
		want  bool
	}{
		{name: "older build is before cutoff", built: "2026-09-20T10:00:00Z", want: true},
		{name: "newer build is not before cutoff", built: "2026-10-01T10:00:00Z"},
		{name: "missing label counts as before", want: true},
		{name: "invalid label counts as before", built: "not-a-time", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := ImageInfo{Built: tc.built}.BuiltBefore(cutoff)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestImageRefreshReason(t *testing.T) {
	maxAge := 7 * 24 * time.Hour
	fresh := formatLabelTime(time.Now())
	stale := formatLabelTime(time.Now().Add(-maxAge - time.Hour))

	for _, tc := range []struct {
		name   string
		info   *ImageInfo
		maxAge time.Duration
		want   string
	}{
		{name: "missing image", maxAge: maxAge, want: "image missing"},
		{name: "different CLI version", info: &ImageInfo{CLIVersion: "v0.0.0", Built: fresh}, maxAge: maxAge, want: "built by a different agentic version"},
		{name: "older than max age", info: &ImageInfo{CLIVersion: buildinfo.Version, Built: stale}, maxAge: maxAge, want: "older than 7 days"},
		{name: "zero max age skips the age check", info: &ImageInfo{CLIVersion: buildinfo.Version, Built: stale}},
		{name: "current image", info: &ImageInfo{CLIVersion: buildinfo.Version, Built: fresh}, maxAge: maxAge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			reason := ImageRefreshReason(tc.info, tc.maxAge)

			// Assert
			assert.Equal(t, tc.want, reason)
		})
	}
}

func TestBuiltTools(t *testing.T) {
	// Arrange
	images := []*ImageInfo{{Tool: "claude"}, {Tool: "claude"}, {Tool: "copilot"}}

	// Act
	result := BuiltTools(images)

	// Assert
	assert.Equal(t, map[string]bool{"claude": true, "copilot": true}, result)
}
