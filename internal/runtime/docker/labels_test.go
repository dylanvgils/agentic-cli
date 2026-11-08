package docker

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLabel_buildsFlag(t *testing.T) {
	// Act
	result := label("agentic.base", "node@24.0.0")

	// Assert
	assert.Equal(t, "--label=agentic.base=node@24.0.0", result)
}

func TestNewCacheBust(t *testing.T) {
	// Arrange
	first := NewCacheBust()

	// Act
	second := NewCacheBust()

	// Assert
	assert.NotEmpty(t, first)
	assert.NotEqual(t, first, second)
}

func TestBuildBaseLabel(t *testing.T) {
	for _, tc := range []struct {
		name     string
		extras   []string
		versions map[string]string
		want     string
	}{
		{name: "single extra with version", extras: []string{"node"}, versions: map[string]string{"node": "24.0.0"}, want: "node@24.0.0"},
		{name: "single extra version missing", extras: []string{"node"}, want: "node"},
		{name: "no extras returns empty string"},
		{name: "multiple extras with versions", extras: []string{"java", "python"}, versions: map[string]string{"java": "21.0.1", "python": ""}, want: "java@21.0.1,python"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := buildBaseLabel(tc.extras, tc.versions)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestBuildVersionArgsLabel(t *testing.T) {
	for _, tc := range []struct {
		name      string
		overrides map[string]string
		want      string
	}{
		{name: "uses overrides when given", overrides: map[string]string{"node": "22", "java": "17"}, want: "node@22,java@17"},
		{name: "falls back to embedded defaults", want: "node@" + tools.DefaultVersions.Node + ",java@" + tools.DefaultVersions.Java},
		{name: "mixes overrides and defaults", overrides: map[string]string{"java": "17"}, want: "node@" + tools.DefaultVersions.Node + ",java@17"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := buildVersionArgsLabel([]string{"node", "java"}, tc.overrides)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestRecoverVersionArgs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		label string
		want  map[string]string
	}{
		{name: "parses layer name and version pairs", label: "node@24,java@17", want: map[string]string{"node": "24", "java": "17"}},
		{name: "skips entries without a version", label: "node@24,java", want: map[string]string{"node": "24"}},
		{name: "empty string returns empty map", want: map[string]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := RecoverVersionArgs(tc.label)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestRecoverApt(t *testing.T) {
	for _, tc := range []struct {
		name  string
		label string
		want  []string
	}{
		{name: "splits comma-separated packages", label: "make,gcc,jq", want: []string{"make", "gcc", "jq"}},
		{name: "trims spaces", label: "make, gcc", want: []string{"make", "gcc"}},
		{name: "empty string returns nil"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := RecoverApt(tc.label)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestRecoverExtras(t *testing.T) {
	for _, tc := range []struct {
		name  string
		label string
		want  []string
	}{
		{name: "strips versions from entries", label: "node@24.0.0,java@21.0.1,python@3.11", want: []string{"node", "java", "python"}},
		{name: "node only", label: "node@24.0.0", want: []string{"node"}},
		{name: "empty string returns nil"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := RecoverExtras(tc.label)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestRecoveredBaseOverride(t *testing.T) {
	for _, tc := range []struct {
		name string
		base string
		opts tools.BuildOptions
		want []string
	}{
		{name: "recovers from label when override is empty", base: "node@24,java@21", want: []string{"node", "java"}},
		{name: "explicit override wins over label", base: "node@24", opts: tools.BuildOptions{BaseOverride: []string{"go"}}, want: []string{"go"}},
		{name: "exact mode skips recovery even when empty", base: "node@24", opts: tools.BuildOptions{BaseOverride: []string{}, BaseExact: true}, want: []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := RecoveredBaseOverride(&ImageInfo{Base: tc.base}, tc.opts)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestRecoveredAptPackages(t *testing.T) {
	for _, tc := range []struct {
		name          string
		apt           string
		exact         bool
		wantMerged    []string
		wantRecovered []string
	}{
		{name: "recovers from label and merges with opts", apt: "make,gcc", wantMerged: []string{"make", "gcc", "cmake"}, wantRecovered: []string{"make", "gcc"}},
		{name: "no label to recover from", wantMerged: []string{"cmake"}},
		{name: "exact mode skips recovery", apt: "make", exact: true, wantMerged: []string{"cmake"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			merged, recovered := RecoveredAptPackages(&ImageInfo{Apt: tc.apt}, tools.BuildOptions{AptPackages: []string{"cmake"}, AptExact: tc.exact})

			// Assert
			assert.Equal(t, tc.wantMerged, merged)
			assert.Equal(t, tc.wantRecovered, recovered)
		})
	}
}

func TestPullIsFresh(t *testing.T) {
	for _, tc := range []struct {
		name  string
		label string
		want  bool
	}{
		{name: "timestamp within interval is fresh", label: formatLabelTime(time.Now()), want: true},
		{name: "timestamp older than interval is not fresh", label: formatLabelTime(time.Now().Add(-25 * time.Hour))},
		{name: "empty label is not fresh"},
		{name: "unparseable label is not fresh", label: "not-a-timestamp"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := PullIsFresh(tc.label, 24*time.Hour)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
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
		LabelCustomInstallsHash,
		LabelBuilt,
		LabelPulled,
		LabelCLIVersion,
		LabelCacheBust,
	}, keys)
}

func TestStampLabels(t *testing.T) {
	// Arrange
	client := newTestClient()
	var capturedArgs []string
	stubDockerRunWithStdin(t, client, func(_ io.Reader, args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	})
	info := ImageInfo{
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

	// Act
	client.stampLabels("agentic-claude", info)

	// Assert - the project label and every non-empty field, tagged back onto the image without a build context
	assert.Equal(t, []string{
		"build",
		"--label=" + LabelProject + "=" + LabelProjectVal,
		"--label=" + LabelNamespace + "=agentic",
		"--label=" + LabelTool + "=claude",
		"--label=" + LabelToolVersion + "=1.2.3",
		"--label=" + LabelBase + "=node@24.0.0",
		"--label=" + LabelVersionArgs + "=node@24",
		"--label=" + LabelApt + "=make,gcc",
		"--label=" + LabelCustomInstalls + "=helm",
		"--label=" + LabelBuilt + "=2026-08-21T00:00:00Z",
		"--label=" + LabelPulled + "=2026-08-20T00:00:00Z",
		"--label=" + LabelCLIVersion + "=1.0.0",
		"--tag=agentic-claude",
		"-",
	}, capturedArgs)
}

func TestStampImageLabels(t *testing.T) {
	client := newTestClient()

	var capturedArgs []string
	stubDockerRunWithStdin(t, client, func(_ io.Reader, args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	})

	t.Run("maps the build options and detected versions to labels", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "21.0.1\n", nil)
		installs := []config.RCCustomInstall{
			{Name: "helm", Run: []string{"echo helm"}},
			{Name: "golangci-lint", Run: []string{"echo lint"}},
		}
		opts := tools.BuildOptions{
			BaseOverride:   []string{"java"},
			Versions:       map[string]string{"java": "17"},
			AptPackages:    []string{"make", "gcc"},
			CustomInstalls: installs,
			CacheBust:      "2026-08-21T07:18:37Z",
		}

		// Act
		client.stampImageLabels("myproject-claude", "claude", opts)

		// Assert
		assert.Contains(t, capturedArgs, "--label="+LabelNamespace+"=myproject")
		assert.Contains(t, capturedArgs, "--label="+LabelTool+"=claude")
		assert.Contains(t, capturedArgs, "--label="+LabelToolVersion+"=21.0.1")
		assert.Contains(t, capturedArgs, "--label="+LabelBase+"=java@21.0.1")
		assert.Contains(t, capturedArgs, "--label="+LabelVersionArgs+"="+tools.BaseLayer+"@"+tools.DefaultVersions.Debian+",java@17")
		assert.Contains(t, capturedArgs, "--label="+LabelApt+"=make,gcc")
		assert.Contains(t, capturedArgs, "--label="+LabelCustomInstalls+"=helm,golangci-lint")
		assert.Contains(t, capturedArgs, "--label="+LabelCustomInstallsHash+"="+config.CustomInstallsHash(installs))
		assert.Contains(t, capturedArgs, "--label="+LabelCacheBust+"=2026-08-21T07:18:37Z")
		assert.True(t, hasArgWithPrefix(capturedArgs, "--label="+LabelBuilt+"="))
		assert.True(t, hasArgWithPrefix(capturedArgs, "--label="+LabelCLIVersion+"="))
	})

	t.Run("omits labels for unset options and a failed version detection", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", fmt.Errorf("version script not found"))

		// Act
		client.stampImageLabels("agentic-claude", "claude", tools.BuildOptions{})

		// Assert
		for _, key := range []string{LabelToolVersion, LabelApt, LabelCustomInstalls, LabelCacheBust} {
			assert.False(t, hasArgWithPrefix(capturedArgs, "--label="+key+"="), "unexpected %s label in args", key)
		}
	})
}

func TestRestampImage(t *testing.T) {
	// Arrange
	client := newTestClient()
	var capturedArgs []string
	stubDockerRunWithStdin(t, client, func(_ io.Reader, args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	})
	info := ImageInfo{
		Namespace:  "agentic",
		Tool:       "claude",
		Version:    "1.2.3",
		Apt:        "make,gcc",
		Base:       "node@24.0.0",
		Built:      "2020-01-01T00:00:00Z",
		CLIVersion: "old",
	}

	// Act
	client.RestampImage("agentic-claude", info)

	// Assert
	require.NotEmpty(t, capturedArgs, "expected a relabel-only build via runWithStdin")
	assert.Contains(t, capturedArgs, "--label=agentic.tool=claude")
	assert.Contains(t, capturedArgs, "--label=agentic.namespace=agentic")
	assert.Contains(t, capturedArgs, "--label=agentic.apt=make,gcc", "unchanged labels must be carried forward")
	assert.Contains(t, capturedArgs, "--label=agentic.base=node@24.0.0", "unchanged labels must be carried forward")
	assert.NotContains(t, capturedArgs, "--label=agentic.built=2020-01-01T00:00:00Z", "expected a fresh agentic.built label")
	assert.NotContains(t, capturedArgs, "--label=agentic.version=old", "expected the current CLI version label")
	assert.True(t, hasArgWithPrefix(capturedArgs, "--label=agentic.built="))
	assert.True(t, hasArgWithPrefix(capturedArgs, "--label=agentic.version="))
}
