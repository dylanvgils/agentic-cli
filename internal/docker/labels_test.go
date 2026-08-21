package docker

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
)

func TestLabel_buildsFlag(t *testing.T) {
	// Act
	result := label("agentic.base", "node@24.0.0")

	// Assert
	assert.Equal(t, "--label=agentic.base=node@24.0.0", result)
}

func TestNewCacheBust(t *testing.T) {
	t.Run("returns non-empty value", func(t *testing.T) {
		// Act
		result := NewCacheBust()

		// Assert
		assert.NotEmpty(t, result)
	})

	t.Run("differs between calls", func(t *testing.T) {
		// Arrange
		first := NewCacheBust()

		// Act
		second := NewCacheBust()

		// Assert
		assert.NotEqual(t, first, second)
	})
}

func TestBuildBaseLabel(t *testing.T) {
	t.Run("single extra with version", func(t *testing.T) {
		// Act
		result := buildBaseLabel([]string{"node"}, map[string]string{"node": "24.0.0"})

		// Assert
		assert.Equal(t, "node@24.0.0", result)
	})

	t.Run("single extra version missing", func(t *testing.T) {
		// Act
		result := buildBaseLabel([]string{"node"}, nil)

		// Assert
		assert.Equal(t, "node", result)
	})

	t.Run("no extras returns empty string", func(t *testing.T) {
		// Act
		result := buildBaseLabel(nil, nil)

		// Assert
		assert.Equal(t, "", result)
	})

	t.Run("multiple extras with versions", func(t *testing.T) {
		// Arrange
		extraVersions := map[string]string{"java": "21.0.1", "python": ""}

		// Act
		result := buildBaseLabel([]string{"java", "python"}, extraVersions)

		// Assert
		assert.Equal(t, "java@21.0.1,python", result)
	})
}

func TestBuildVersionArgsLabel(t *testing.T) {
	t.Run("uses overrides when given", func(t *testing.T) {
		// Act
		result := buildVersionArgsLabel([]string{"node", "java"}, map[string]string{"node": "22", "java": "17"})

		// Assert
		assert.Equal(t, "node@22,java@17", result)
	})

	t.Run("falls back to embedded defaults", func(t *testing.T) {
		// Act
		result := buildVersionArgsLabel([]string{"node", "java"}, nil)

		// Assert
		assert.Equal(t, "node@"+tools.DefaultVersions.Node+",java@"+tools.DefaultVersions.Java, result)
	})

	t.Run("mixes overrides and defaults", func(t *testing.T) {
		// Act
		result := buildVersionArgsLabel([]string{"node", "java"}, map[string]string{"java": "17"})

		// Assert
		assert.Equal(t, "node@"+tools.DefaultVersions.Node+",java@17", result)
	})
}

func TestRecoverVersionArgs(t *testing.T) {
	t.Run("parses layer name and version pairs", func(t *testing.T) {
		// Act
		result := RecoverVersionArgs("node@24,java@17")

		// Assert
		assert.Equal(t, map[string]string{"node": "24", "java": "17"}, result)
	})

	t.Run("skips entries without a version", func(t *testing.T) {
		// Act
		result := RecoverVersionArgs("node@24,java")

		// Assert
		assert.Equal(t, map[string]string{"node": "24"}, result)
	})

	t.Run("empty string returns empty map", func(t *testing.T) {
		// Act
		result := RecoverVersionArgs("")

		// Assert
		assert.Empty(t, result)
	})
}

func TestRecoverApt(t *testing.T) {
	t.Run("splits comma-separated packages", func(t *testing.T) {
		// Act
		result := RecoverApt("make,gcc,jq")

		// Assert
		assert.Equal(t, []string{"make", "gcc", "jq"}, result)
	})

	t.Run("trims spaces", func(t *testing.T) {
		// Act
		result := RecoverApt("make, gcc")

		// Assert
		assert.Equal(t, []string{"make", "gcc"}, result)
	})

	t.Run("empty string returns nil", func(t *testing.T) {
		// Act
		result := RecoverApt("")

		// Assert
		assert.Nil(t, result)
	})
}

func TestPullIsFresh(t *testing.T) {
	t.Run("timestamp within interval is fresh", func(t *testing.T) {
		// Arrange
		label := formatLabelTime(time.Now())

		// Act
		result := PullIsFresh(label, 24*time.Hour)

		// Assert
		assert.True(t, result)
	})

	t.Run("timestamp older than interval is not fresh", func(t *testing.T) {
		// Arrange
		label := formatLabelTime(time.Now().Add(-25 * time.Hour))

		// Act
		result := PullIsFresh(label, 24*time.Hour)

		// Assert
		assert.False(t, result)
	})

	t.Run("empty label is not fresh", func(t *testing.T) {
		// Act
		result := PullIsFresh("", 24*time.Hour)

		// Assert
		assert.False(t, result)
	})

	t.Run("unparseable label is not fresh", func(t *testing.T) {
		// Act
		result := PullIsFresh("not-a-timestamp", 24*time.Hour)

		// Assert
		assert.False(t, result)
	})
}

func TestImageLabelPairs(t *testing.T) {
	// Act
	pairs := imageLabelPairs(ImageInfo{})
	keys := make([]string, len(pairs))
	for i, p := range pairs {
		keys[i] = p.key
	}

	// Assert - a new LabelXxx constant must be added here to be picked up by
	// stampLabels; this fails loudly instead of letting it silently drift.
	assert.ElementsMatch(t, []string{
		LabelNamespace,
		LabelTool,
		LabelToolVersion,
		LabelBase,
		LabelVersionArgs,
		LabelApt,
		LabelCustomInstalls,
		LabelBuilt,
		LabelPulled,
		LabelCLIVersion,
		LabelCacheBust,
	}, keys)
}

func TestStampLabels(t *testing.T) {
	var capturedArgs []string
	origStdin := dockerRunStdin
	dockerRunStdin = func(_ io.Reader, args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	}
	t.Cleanup(func() { dockerRunStdin = origStdin })

	fullInfo := ImageInfo{
		Namespace:      "agentic",
		Tool:           "claude",
		Version:        "1.2.3",
		Base:           "node@24.0.0",
		VersionArgs:    "node@24",
		Apt:            "make,gcc",
		CustomInstalls: "helm",
		Built:          "2026-08-21T00:00:00Z",
		Pulled:         "2026-08-20T00:00:00Z",
		CLIVersion:     "1.0.0",
	}

	t.Run("always includes project label", func(t *testing.T) {
		// Act
		stampLabels("agentic-claude", ImageInfo{})

		// Assert
		assert.Contains(t, capturedArgs, "--label="+LabelProject+"="+LabelProjectVal)
	})

	t.Run("includes every non-empty field in info", func(t *testing.T) {
		// Act
		stampLabels("agentic-claude", fullInfo)

		// Assert
		assert.Contains(t, capturedArgs, "--label="+LabelNamespace+"=agentic")
		assert.Contains(t, capturedArgs, "--label="+LabelTool+"=claude")
		assert.Contains(t, capturedArgs, "--label="+LabelToolVersion+"=1.2.3")
		assert.Contains(t, capturedArgs, "--label="+LabelBase+"=node@24.0.0")
		assert.Contains(t, capturedArgs, "--label="+LabelVersionArgs+"=node@24")
		assert.Contains(t, capturedArgs, "--label="+LabelApt+"=make,gcc")
		assert.Contains(t, capturedArgs, "--label="+LabelCustomInstalls+"=helm")
		assert.Contains(t, capturedArgs, "--label="+LabelBuilt+"=2026-08-21T00:00:00Z")
		assert.Contains(t, capturedArgs, "--label="+LabelPulled+"=2026-08-20T00:00:00Z")
		assert.Contains(t, capturedArgs, "--label="+LabelCLIVersion+"=1.0.0")
	})

	t.Run("omits the label for an empty field", func(t *testing.T) {
		// Act - Pulled left blank
		stampLabels("agentic-claude", ImageInfo{Namespace: "agentic", Tool: "claude"})

		// Assert
		assert.False(t, hasArgWithPrefix(capturedArgs, "--label="+LabelPulled+"="),
			"unexpected %s label in args", LabelPulled)
	})

	t.Run("tags back the same image without a build context", func(t *testing.T) {
		// Act
		stampLabels("agentic-claude", fullInfo)

		// Assert
		assert.Contains(t, capturedArgs, "--tag=agentic-claude")
		assert.Equal(t, "-", capturedArgs[len(capturedArgs)-1])
	})
}

func TestStampImageLabels(t *testing.T) {
	var capturedArgs []string
	origStdin := dockerRunStdin
	dockerRunStdin = func(_ io.Reader, args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	}
	t.Cleanup(func() { dockerRunStdin = origStdin })

	t.Run("includes namespace and tool labels derived from the image name", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", nil)

		// Act
		stampImageLabels("myproject-claude", "claude", nil, nil, nil, nil, "")

		// Assert
		assert.Contains(t, capturedArgs, "--label="+LabelNamespace+"=myproject")
		assert.Contains(t, capturedArgs, "--label="+LabelTool+"=claude")
	})

	t.Run("includes a fresh built timestamp and CLI version", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", nil)

		// Act
		stampImageLabels("agentic-claude", "claude", nil, nil, nil, nil, "")

		// Assert
		assert.True(t, hasArgWithPrefix(capturedArgs, "--label="+LabelBuilt+"="),
			"expected --%s label in args", LabelBuilt)
		assert.True(t, hasArgWithPrefix(capturedArgs, "--label="+LabelCLIVersion+"="),
			"expected --%s label in args", LabelCLIVersion)
	})

	t.Run("includes apt label with packages", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", nil)

		// Act
		stampImageLabels("agentic-claude", "claude", nil, []string{"make", "gcc"}, nil, nil, "")

		// Assert
		assert.Contains(t, capturedArgs, "--label="+LabelApt+"=make,gcc")
	})

	t.Run("includes custom installs label with names", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", nil)

		// Act
		stampImageLabels("agentic-claude", "claude", nil, nil, nil, []string{"helm", "golangci-lint"}, "")

		// Assert
		assert.Contains(t, capturedArgs, "--label="+LabelCustomInstalls+"=helm,golangci-lint")
	})

	t.Run("includes base label", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "21.0.1\n", nil)

		// Act
		stampImageLabels("agentic-claude", "claude", []string{"java"}, nil, nil, nil, "")

		// Assert
		assert.Contains(t, capturedArgs, "--label="+LabelBase+"=java@21.0.1")
	})

	t.Run("includes version-args label", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", nil)

		// Act
		stampImageLabels("agentic-claude", "claude", []string{"java"}, nil, map[string]string{"java": "17"}, nil, "")

		// Assert
		found := false
		for _, a := range capturedArgs {
			if strings.HasPrefix(a, "--label="+LabelVersionArgs+"=") {
				found = true
				break
			}
		}
		assert.True(t, found, "expected --%s label in args", LabelVersionArgs)
	})

	t.Run("includes tool version label when detected", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "1.2.3\n", nil)

		// Act
		stampImageLabels("agentic-claude", "claude", nil, nil, nil, nil, "")

		// Assert
		assert.Contains(t, capturedArgs, "--label="+LabelToolVersion+"=1.2.3")
	})

	t.Run("omits tool version label when detection fails", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", fmt.Errorf("version script not found"))

		// Act
		stampImageLabels("agentic-claude", "claude", nil, nil, nil, nil, "")

		// Assert
		for _, a := range capturedArgs {
			assert.False(t, strings.HasPrefix(a, "--label="+LabelToolVersion+"="),
				"unexpected %s label in args: %s", LabelToolVersion, a)
		}
	})

	t.Run("records the cachebust value the tool stage was built with", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", nil)

		// Act
		stampImageLabels("agentic-claude", "claude", nil, nil, nil, nil, "2026-08-21T07:18:37Z")

		// Assert
		assert.Contains(t, capturedArgs, "--label="+LabelCacheBust+"=2026-08-21T07:18:37Z")
	})

	t.Run("omits cachebust label when built without one", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", nil)

		// Act
		stampImageLabels("agentic-claude", "claude", nil, nil, nil, nil, "")

		// Assert
		assert.False(t, hasArgWithPrefix(capturedArgs, "--label="+LabelCacheBust+"="),
			"unexpected %s label in args", LabelCacheBust)
	})
}

func TestRecoverExtras(t *testing.T) {
	t.Run("strips versions from entries", func(t *testing.T) {
		// Act
		result := RecoverExtras("node@24.0.0,java@21.0.1")

		// Assert
		assert.Equal(t, []string{"node", "java"}, result)
	})

	t.Run("multiple extras", func(t *testing.T) {
		// Act
		result := RecoverExtras("node@24.0.0,java@21.0.1,python@3.11")

		// Assert
		assert.Equal(t, []string{"node", "java", "python"}, result)
	})

	t.Run("node only", func(t *testing.T) {
		// Act
		result := RecoverExtras("node@24.0.0")

		// Assert
		assert.Equal(t, []string{"node"}, result)
	})

	t.Run("empty string returns nil", func(t *testing.T) {
		// Act
		result := RecoverExtras("")

		// Assert
		assert.Nil(t, result)
	})
}
