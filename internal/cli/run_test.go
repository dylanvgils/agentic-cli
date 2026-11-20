package cli

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/run"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runTool(t *testing.T) {
	// Keep the repo's own .agenticrc.toml out of the test
	t.Chdir(t.TempDir())

	t.Run("no args prints help", func(t *testing.T) {
		// Arrange
		get := captureToolRun(t)

		// Act
		err := runTool(runToolCmd, []string{})

		// Assert
		require.NoError(t, err)
		rs := get()
		assert.Empty(t, rs.Image, "the tool should not run when no args given")
	})

	t.Run("runs the tool's image with its args", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		withTempToolHome(t)
		get := captureToolRun(t)
		stubDocker(t, &fakeDocker{context: "test-context"})
		spawned := stubWatchdogs(t)

		// Act
		err := runTool(runToolCmd, []string{"claude", "--dangerously-skip-permissions"})

		// Assert
		require.NoError(t, err)
		rs := get()
		assert.Equal(t, "agentic-claude", rs.Image)
		assert.Equal(t, []string{"--dangerously-skip-permissions"}, rs.ToolArgs)
		assert.Equal(t, []string{"test-context"}, spawned.contexts)
		require.Len(t, spawned.spawned, 1)
		assert.Equal(t, rs.RunID, spawned.spawned[0].ID)
	})
}

func Test_parseArgs(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want parsedArgs
	}{
		{
			name: "tool name and image name",
			args: []string{"claude"},
			want: parsedArgs{toolName: "claude", imageName: "agentic-claude", toolArgs: []string{}},
		},
		{
			name: "tool args",
			args: []string{"claude", "--flag", "value"},
			want: parsedArgs{toolName: "claude", imageName: "agentic-claude", toolArgs: []string{"--flag", "value"}},
		},
		{
			name: "dash dash sets skip entrypoint",
			args: []string{"claude", "--", "bash", "-c", "echo hi"},
			want: parsedArgs{toolName: "claude", imageName: "agentic-claude", toolArgs: []string{"bash", "-c", "echo hi"}, skipEntrypoint: true},
		},
		{
			name: "dash dash no trailing args",
			args: []string{"claude", "--"},
			want: parsedArgs{toolName: "claude", imageName: "agentic-claude", toolArgs: []string{}, skipEntrypoint: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result, err := parseArgs(tc.args, "agentic")

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, result)
		})
	}

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		_, err := parseArgs([]string{"bogus"}, "agentic")

		// Assert
		assert.ErrorContains(t, err, "bogus")
	})
}

func Test_runRequest(t *testing.T) {
	// Arrange
	withTempToolHome(t)
	stubFlag(t, runToolCmd, "trust-dir", "true")
	stubFlag(t, runToolCmd, "dry-run", "true")
	stubFlag(t, runToolCmd, "volume", "/example.test/a:/a")
	stubFlag(t, runToolCmd, "env", "FOO=bar")
	stubFlag(t, runToolCmd, "registry", "registry.example.test")
	stubFlag(t, runToolCmd, "proxy", "true")
	stubFlag(t, runToolCmd, "runtime", "microsandbox")
	inv := invocation{
		parsedArgs: parsedArgs{toolName: "claude", imageName: "agentic-claude", skipEntrypoint: true},
		project: config.Project{
			WorkDir: "/example.test/project",
			RC:      &config.AgenticRC{},
		},
	}

	// Act
	req, err := runRequest(runToolCmd, inv)

	// Assert - run-only flags land next to the shared runtimeFlags
	require.NoError(t, err)
	assert.Equal(t, run.Target{ToolName: "claude", ImageName: "agentic-claude", SkipEntrypoint: true}, req.Target)
	assert.Equal(t, "/example.test/project", req.Project.WorkDir)
	assert.True(t, req.Flags.TrustDir)
	assert.True(t, req.Flags.DryRun)
	assert.True(t, req.Flags.Proxy.ProxyFlag)
	assert.Equal(t, "registry.example.test", req.Flags.Registry)
	assert.Equal(t, []string{"/example.test/a:/a"}, req.Flags.Volumes)
	assert.Equal(t, []string{"FOO=bar"}, req.Flags.Env)
	assert.Equal(t, sandbox.RuntimeMicrosandbox, req.Flags.Runtime)
}

func Test_toolUpdates_Offer(t *testing.T) {
	// Arrange
	iostreamstest.StubErr(t)
	var built []string
	stubDocker(t, &fakeDocker{
		inspectImage: func(name string) (*docker.ImageInfo, error) {
			return &docker.ImageInfo{Image: name, Version: "1.0.0"}, nil
		},
		buildTool: func(tool, image string, _ tools.BuildOptions) error {
			built = []string{tool, image}
			return nil
		},
	})
	stubUpstream(t, "2.0.0", true, true)
	u := toolUpdates{docker: dockerClient, upstream: upstreamClient}
	req := run.ToolUpdate{Home: t.TempDir(), RC: &config.AgenticRC{}, Tool: "claude", Image: "agentic-claude"}

	// Act
	err := u.Offer(req, func(string, string, string) bool { return true })

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"claude", "agentic-claude"}, built)
}
