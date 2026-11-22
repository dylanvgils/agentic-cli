package build

import (
	"errors"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config/agentichome"
	"github.com/dylanvgils/agentic-cli/internal/config/agenticrc"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Resolve(t *testing.T) {
	t.Run("names each tool's image in the namespace", func(t *testing.T) {
		// Arrange
		opts := tools.BuildOptions{NoCache: true}

		// Act
		plan, err := New(&fakeDocker{}, t.TempDir()).Resolve([]string{"claude", "copilot"}, "agentic", opts)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, Plan{
			Targets: []Target{
				{Name: "claude", Image: "agentic-claude"},
				{Name: "copilot", Image: "agentic-copilot"},
			},
			Opts: opts,
		}, plan)
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		_, err := New(&fakeDocker{}, t.TempDir()).Resolve([]string{"nonexistent"}, "agentic", tools.BuildOptions{})

		// Assert
		assert.ErrorContains(t, err, "unknown tool")
	})
}

func TestService_DryRun(t *testing.T) {
	t.Run("prints the labelled dockerfile without building", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{}
		built := stubBuilds(d, nil)
		errOut := iostreamstest.StubErr(t)
		out := iostreamstest.StubOut(t)

		// Act
		err := New(d, t.TempDir()).DryRun(newPlan(tools.BuildOptions{}, "claude"))

		// Assert
		require.NoError(t, err)
		assert.Empty(t, *built)
		assert.Contains(t, out.String(), "FROM")
		assert.NotContains(t, out.String(), "AS proxy", "the proxy image builds separately")
		assert.Equal(t, "agentic: claude Dockerfile:\n\n", errOut.String())
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		err := New(&fakeDocker{}, t.TempDir()).DryRun(newPlan(tools.BuildOptions{}, "nonexistent"))

		// Assert
		assert.ErrorContains(t, err, "unknown tool")
	})
}

func TestService_Apply(t *testing.T) {
	t.Run("builds each tool, then prunes and records the checks", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		pruned := false
		d := &fakeDocker{pruneImages: func() (string, error) { pruned = true; return "", nil }}
		built := stubBuilds(d, nil)

		// Act
		var err error
		out := iostreamstest.CaptureErr(t, func() {
			err = New(d, home).Apply(newPlan(tools.BuildOptions{}, "claude", "copilot", "opencode"))
		})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"claude", "copilot", "opencode"}, *built)
		assert.Equal(t, "=> agentic-claude\n=> agentic-copilot\n=> agentic-opencode\n", out)
		assert.True(t, pruned)
		state, err := agentichome.LoadState(home)
		require.NoError(t, err)
		assert.Contains(t, state.LastToolVersionCheck, "claude")
		assert.Contains(t, state.LastToolVersionCheck, "opencode")
	})

	t.Run("stores custom installs before building", func(t *testing.T) {
		// Arrange
		iostreamstest.StubErr(t)
		home := t.TempDir()
		installs := []agenticrc.CustomInstall{{Name: "helm", Run: []string{"echo helm"}}}

		// Act
		err := New(&fakeDocker{}, home).Apply(newPlan(tools.BuildOptions{CustomInstalls: installs}, "claude"))

		// Assert
		require.NoError(t, err)
		stored, ok := agentichome.LoadCustomInstalls(home, agentichome.CustomInstallsHash(installs))
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
	} {
		t.Run("reports "+tc.name, func(t *testing.T) {
			// Act
			var err error
			out := iostreamstest.CaptureErr(t, func() { err = New(&fakeDocker{}, t.TempDir()).Apply(newPlan(tc.opts, "claude")) })

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, out)
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
		err := New(d, home).Apply(newPlan(tools.BuildOptions{}, "claude", "copilot", "opencode"))

		// Assert
		require.ErrorContains(t, err, "build failed")
		assert.Equal(t, []string{"claude"}, *built)
		assert.False(t, pruned)
		state, err := agentichome.LoadState(home)
		require.NoError(t, err)
		assert.NotContains(t, state.LastToolVersionCheck, "claude")
	})

	t.Run("a failed prune does not fail the build", func(t *testing.T) {
		// Arrange
		iostreamstest.StubErr(t)
		d := &fakeDocker{pruneImages: func() (string, error) { return "", errors.New("prune failed") }}

		// Act
		err := New(d, t.TempDir()).Apply(newPlan(tools.BuildOptions{}, "claude"))

		// Assert
		require.NoError(t, err)
	})
}
