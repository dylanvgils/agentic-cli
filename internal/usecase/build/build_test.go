package build

import (
	"errors"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDryRun(t *testing.T) {
	t.Run("prints the labelled dockerfile without building", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{}
		built := stubBuilds(d, nil)
		errOut := iostreamstest.StubErr(t)
		out := iostreamstest.StubOut(t)

		// Act
		err := New(d, t.TempDir()).DryRun([]string{"claude"}, tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
		assert.Empty(t, *built)
		assert.Contains(t, out.String(), "FROM")
		assert.NotContains(t, out.String(), "AS proxy", "the proxy image builds separately")
		assert.Equal(t, "agentic: claude Dockerfile:\n\n", errOut.String())
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		err := New(&fakeDocker{}, t.TempDir()).DryRun([]string{"nonexistent"}, tools.BuildOptions{})

		// Assert
		assert.ErrorContains(t, err, "unknown tool")
	})
}

func TestApply(t *testing.T) {
	t.Run("builds each tool, then prunes and records the checks", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		pruned := false
		d := &fakeDocker{pruneImages: func() (string, error) { pruned = true; return "", nil }}
		built := stubBuilds(d, nil)

		// Act
		var err error
		out := iostreamstest.CaptureErr(t, func() {
			err = New(d, home).Apply([]string{"claude", "copilot", "opencode"}, "agentic", tools.BuildOptions{})
		})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"claude", "copilot", "opencode"}, *built)
		assert.Equal(t, "agentic: building 3 image(s): agentic-claude, agentic-copilot, agentic-opencode\n=> agentic-claude\n=> agentic-copilot\n=> agentic-opencode\n", out)
		assert.True(t, pruned)
		state, err := config.LoadState(home)
		require.NoError(t, err)
		assert.Contains(t, state.LastToolVersionCheck, "claude")
		assert.Contains(t, state.LastToolVersionCheck, "opencode")
	})

	t.Run("stores custom installs before building", func(t *testing.T) {
		// Arrange
		iostreamstest.StubErr(t)
		home := t.TempDir()
		installs := []config.RCCustomInstall{{Name: "helm", Run: []string{"echo helm"}}}

		// Act
		err := New(&fakeDocker{}, home).Apply([]string{"claude"}, "agentic", tools.BuildOptions{CustomInstalls: installs})

		// Assert
		require.NoError(t, err)
		stored, ok := config.LoadCustomInstalls(home, config.CustomInstallsHash(installs))
		assert.True(t, ok)
		assert.Equal(t, installs, stored)
	})

	for _, tc := range []struct {
		name string
		opts tools.BuildOptions
		want string
	}{
		{name: "nothing but the image without overrides", want: "=> agentic-claude\n"},
		{
			name: "base and apt overrides",
			opts: tools.BuildOptions{BaseOverride: []string{"java", "dotnet"}, AptPackages: []string{"curl", "jq"}},
			want: "=> agentic-claude\n   base: java, dotnet\n   apt: curl, jq\n",
		},
		{
			name: "empty exact base and apt",
			opts: tools.BuildOptions{BaseExact: true, AptExact: true},
			want: "=> agentic-claude\n   base: (none, exact)\n   apt: (none, exact)\n",
		},
	} {
		t.Run("reports "+tc.name, func(t *testing.T) {
			// Act
			var err error
			out := iostreamstest.CaptureErr(t, func() { err = New(&fakeDocker{}, t.TempDir()).Apply([]string{"claude"}, "agentic", tc.opts) })

			// Assert
			require.NoError(t, err)
			assert.Equal(t, "agentic: building 1 image(s): agentic-claude\n"+tc.want, out)
		})
	}

	t.Run("a failed build stops the rest, skips the prune and records no check", func(t *testing.T) {
		// Arrange
		iostreamstest.StubErr(t)
		home := t.TempDir()
		pruned := false
		d := &fakeDocker{pruneImages: func() (string, error) { pruned = true; return "", nil }}
		built := stubBuilds(d, errors.New("build failed"))

		// Act
		err := New(d, home).Apply([]string{"claude", "copilot", "opencode"}, "agentic", tools.BuildOptions{})

		// Assert
		require.ErrorContains(t, err, "build failed")
		assert.Equal(t, []string{"claude"}, *built)
		assert.False(t, pruned)
		state, err := config.LoadState(home)
		require.NoError(t, err)
		assert.NotContains(t, state.LastToolVersionCheck, "claude")
	})

	t.Run("a failed prune does not fail the build", func(t *testing.T) {
		// Arrange
		iostreamstest.StubErr(t)
		d := &fakeDocker{pruneImages: func() (string, error) { return "", errors.New("prune failed") }}

		// Act
		err := New(d, t.TempDir()).Apply([]string{"claude"}, "agentic", tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		err := New(&fakeDocker{}, t.TempDir()).Apply([]string{"nonexistent"}, "agentic", tools.BuildOptions{})

		// Assert
		assert.ErrorContains(t, err, "unknown tool")
	})
}
