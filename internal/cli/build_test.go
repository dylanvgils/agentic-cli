package cli

import (
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runBuild(t *testing.T) {
	// Keep the repo's own .agenticrc.toml out of the test
	t.Chdir(t.TempDir())

	for _, tc := range []struct {
		flag  string
		value string
		want  func(tools.BuildOptions) bool
	}{
		{flag: "no-cache", value: "true", want: func(o tools.BuildOptions) bool { return o.NoCache }},
		{flag: "skip-install-checksum", value: "true", want: func(o tools.BuildOptions) bool { return o.SkipInstallChecksum }},
		{flag: "pull", value: "true", want: func(o tools.BuildOptions) bool { return o.Pull }},
		{flag: "node", value: "22", want: func(o tools.BuildOptions) bool { return o.Versions["node"] == "22" }},
	} {
		t.Run(tc.flag+" flag sets opt", func(t *testing.T) {
			// Arrange
			opts := stubBuildTool(t, nil)
			stubFlag(t, buildCmd, tc.flag, tc.value)

			// Act
			err := runBuild(buildCmd, []string{"claude"})

			// Assert
			require.NoError(t, err)
			assert.True(t, tc.want(*opts))
		})
	}

	t.Run("all flag builds every tool", func(t *testing.T) {
		// Arrange
		var built []string
		stubDocker(t, &fakeDocker{buildTool: func(tool, _ string, _ tools.BuildOptions) error {
			built = append(built, tool)
			return nil
		}})

		stubFlag(t, buildCmd, allFlagName, "true")

		// Act
		err := runBuild(buildCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, tools.Names(), built)
	})

	t.Run("dry run flag prints dockerfile and skips build", func(t *testing.T) {
		// Arrange
		var buildCalled bool
		stubDocker(t, &fakeDocker{buildTool: func(_, _ string, _ tools.BuildOptions) error {
			buildCalled = true
			return nil
		}})

		stubFlag(t, buildCmd, "dry-run", "true")

		// Act
		out := iostreamstest.CaptureOut(t, func() {
			err := runBuild(buildCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.False(t, buildCalled)
		assert.Contains(t, out, "FROM")
	})

	t.Run("invalid project config fails fast with a clear error", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		writeRC(t, dir, "not valid toml [[[")
		rcPath := filepath.Join(dir, ".agenticrc.toml")
		t.Chdir(dir)

		// Act
		err := runBuild(buildCmd, []string{"claude"})

		// Assert
		assert.ErrorContains(t, err, rcPath)
	})
}
