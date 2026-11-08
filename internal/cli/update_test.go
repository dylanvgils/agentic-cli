package cli

import (
	"fmt"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/usecase/update"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runUpdate(t *testing.T) {
	// Keep the repo's own .agenticrc.toml out of the test
	t.Chdir(t.TempDir())

	stubUpstream(t, "", false, false)

	built := &docker.ImageInfo{Version: "1.0.0"}
	recentlyPulled := &docker.ImageInfo{Version: "1.0.0", Pulled: time.Now().UTC().Format(time.RFC3339)}

	for _, tc := range []struct {
		name  string
		info  *docker.ImageInfo
		flags map[string]string
		want  func(tools.BuildOptions) bool
	}{
		{name: "pull flag defaults true", info: built, want: func(o tools.BuildOptions) bool { return o.Pull }},
		{name: "pull flag can be disabled", info: built, flags: map[string]string{"pull": "false"}, want: func(o tools.BuildOptions) bool { return !o.Pull }},
		{name: "automatic pull is throttled after a recent pull", info: recentlyPulled, want: func(o tools.BuildOptions) bool { return !o.Pull }},
		{name: "explicit pull is kept after a recent pull", info: recentlyPulled, flags: map[string]string{"pull": "true"}, want: func(o tools.BuildOptions) bool { return o.Pull }},
		{name: "no cache flag sets opt", info: built, flags: map[string]string{"no-cache": "true"}, want: func(o tools.BuildOptions) bool { return o.NoCache }},
		{
			name:  "skip install checksum flag sets opt",
			info:  built,
			flags: map[string]string{"skip-install-checksum": "true"},
			want:  func(o tools.BuildOptions) bool { return o.SkipInstallChecksum },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			opts := stubBuildTool(t, tc.info)
			for name, value := range tc.flags {
				stubFlag(t, updateCmd, name, value)
			}

			// Act
			err := runUpdate(updateCmd, []string{"claude"})

			// Assert
			require.NoError(t, err)
			assert.True(t, tc.want(*opts))
		})
	}

	t.Run("dry run flag prints dockerfile and skips update", func(t *testing.T) {
		// Arrange
		var updateCalled bool
		stubDocker(t, &fakeDocker{
			buildTool: func(_, _ string, _ tools.BuildOptions) error {
				updateCalled = true
				return nil
			},
		})

		stubFlag(t, updateCmd, "dry-run", "true")

		// Act
		out := iostreamstest.CaptureOut(t, func() {
			err := runUpdate(updateCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.False(t, updateCalled)
		assert.Contains(t, out, "FROM")
	})

	t.Run("stops on first update error", func(t *testing.T) {
		// Arrange
		var updated []string
		stubDocker(t, &fakeDocker{
			buildTool: func(tool, _ string, _ tools.BuildOptions) error {
				updated = append(updated, tool)
				return fmt.Errorf("fail on %s", tool)
			},
			inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil),
			listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
				return []*docker.ImageInfo{
					{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
					{Image: "agentic-copilot", Namespace: "agentic", Tool: "copilot"},
				}, nil
			},
		})
		stubFlag(t, updateCmd, allFlagName, "true")

		// Act
		err := runUpdate(updateCmd, []string{})

		// Assert
		require.Error(t, err)
		assert.Len(t, updated, 1)
	})

	t.Run("all without images prints message", func(t *testing.T) {
		// Arrange
		stubDocker(t, &fakeDocker{})
		stubFlag(t, updateCmd, allFlagName, "true")
		logBuf := iostreamstest.StubErr(t)

		// Act
		err := runUpdate(updateCmd, []string{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logBuf.String(), "agentic: no agentic images found")
	})

	t.Run("all flag updates all images", func(t *testing.T) {
		// Arrange
		logBuf := iostreamstest.StubErr(t)
		var updated []string
		stubDocker(t, &fakeDocker{
			buildTool: func(tool, _ string, _ tools.BuildOptions) error {
				updated = append(updated, tool)
				return nil
			},
			inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil),
			listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
				return []*docker.ImageInfo{
					{Image: "agentic-claude", Namespace: "agentic", Tool: "claude", Base: "node@24"},
					{Image: "work-copilot", Namespace: "work", Tool: "copilot", Base: "node@24"},
				}, nil
			},
		})

		cmd := updateCmd
		stubFlag(t, cmd, allFlagName, "true")

		// Act
		err := runUpdate(cmd, []string{})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"claude", "copilot"}, updated)
		assert.Contains(t, logBuf.String(), "agentic: updating 2 image(s): ")
	})

	for _, tc := range []struct {
		name string
		rc   string
		info *docker.ImageInfo
		flag string
		want func(tools.BuildOptions) bool
	}{
		{
			name: "base-exact overrides rc bases and per-image label recovery",
			rc:   "[build]\nbases = [\"java\"]\n",
			info: &docker.ImageInfo{Version: "1.0.0", Base: "go@1.23"},
			flag: "base-exact",
			want: func(o tools.BuildOptions) bool { return len(o.BaseOverride) == 0 && o.BaseExact },
		},
		{
			name: "apt-exact overrides rc apt and per-image label recovery",
			rc:   "[build]\napt_packages = [\"make\"]\n",
			info: &docker.ImageInfo{Version: "1.0.0", Apt: "cmake"},
			flag: "apt-exact",
			want: func(o tools.BuildOptions) bool { return len(o.AptPackages) == 0 && o.AptExact },
		},
	} {
		t.Run(tc.name+", even when empty", func(t *testing.T) {
			// Arrange - neither the rc's value nor the image's recovered one may survive
			dir := t.TempDir()
			writeRC(t, dir, tc.rc)
			t.Chdir(dir)
			opts := stubBuildTool(t, tc.info)
			stubFlag(t, updateCmd, tc.flag, "")

			// Act
			err := runUpdate(updateCmd, []string{"claude"})

			// Assert
			require.NoError(t, err)
			assert.True(t, tc.want(*opts))
		})
	}
}

func Test_logUpdateSummary(t *testing.T) {
	// Arrange
	logBuf := iostreamstest.StubCombined(t)
	targets := []update.Target{{Name: "claude", Image: "agentic-claude"}, {Name: "copilot", Image: "agentic-copilot"}}

	// Act
	logUpdateSummary(targets)

	// Assert
	assert.Contains(t, logBuf.String(), "updating 2 image(s): agentic-claude, agentic-copilot")
}

// Test_updateOptsFromFlags only checks which flags count as given; the rule itself is covered by TestUpdateOptions in internal/usecase/resolve.
func Test_updateOptsFromFlags(t *testing.T) {
	withTempToolHome(t)
	rc := &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}, AptPackages: []string{"jq"}}}

	t.Run("no flags drops the config bases and apt", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		addBuildFlags(cmd)

		// Act
		opts := updateOptsFromFlags(cmd, rc)

		// Assert
		assert.Nil(t, opts.BaseOverride)
		assert.Nil(t, opts.AptPackages)
	})

	t.Run("each flag pair counts on its own", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		addBuildFlags(cmd)
		require.NoError(t, cmd.Flags().Set("apt-exact", "curl"))

		// Act
		opts := updateOptsFromFlags(cmd, rc)

		// Assert - --apt-exact sets apt only; bases still come from the image
		assert.Nil(t, opts.BaseOverride)
		assert.Equal(t, []string{"curl"}, opts.AptPackages)
	})
}
