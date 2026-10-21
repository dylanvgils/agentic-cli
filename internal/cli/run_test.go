package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/tools"
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

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		err := runTool(runToolCmd, []string{"bogus"})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bogus")
	})

	t.Run("builds image name", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		withTempToolHome(t)
		get := captureToolRun(t)

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		rs := get()
		assert.Equal(t, "agentic-claude", rs.Image)
		assert.Empty(t, rs.ToolArgs)
	})

	t.Run("dind flag enables the sidecar when the image has the docker layer", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		withTempToolHome(t)
		get := captureToolRun(t)
		stubInspectImage(t, &docker.ImageInfo{Image: "agentic-claude", Base: "docker@29.8.2"}, nil)
		stubDocker(t, &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Image: "agentic-claude", Base: "docker@29.8.2"}, nil)})
		stubBuildDindImage(t, func(string, tools.BuildOptions) error { return nil })
		stubFlag(t, runToolCmd, "dind", "true")

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		rs := get()
		assert.True(t, rs.Dind.Enabled)
	})

	t.Run("dind flag without docker layer fails before running", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		withTempToolHome(t)
		get := captureToolRun(t)
		stubFlag(t, runToolCmd, "dind", "true")

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		require.ErrorContains(t, err, "--base docker")
		rs := get()
		assert.Empty(t, rs.Image, "the tool should not run")
	})

	t.Run("passes tool args", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		withTempToolHome(t)
		get := captureToolRun(t)

		// Act
		err := runTool(runToolCmd, []string{"claude", "--dangerously-skip-permissions"})

		// Assert
		require.NoError(t, err)
		rs := get()
		assert.Equal(t, []string{"--dangerously-skip-permissions"}, rs.ToolArgs)
	})

	t.Run("invalid project config fails fast with a clear error", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rcPath := filepath.Join(dir, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(rcPath, []byte("not valid toml [[["), 0o644))
		t.Chdir(dir)

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		assert.ErrorContains(t, err, rcPath)
	})

	t.Run("unknown runtime returns error", func(t *testing.T) {
		// Arrange
		stubFlag(t, runToolCmd, "runtime", "podman")

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		assert.ErrorContains(t, err, `unknown runtime "podman"`)
	})

	t.Run("microsandbox runtime loads the image into microsandbox and runs it there", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		withTempToolHome(t)
		get := captureToolRun(t)
		stubDocker(t, &fakeDocker{
			inspectImage: func(name string) (*docker.ImageInfo, error) {
				return &docker.ImageInfo{Image: name, ID: "abc123"}, nil
			},
			saveImage: func(image string, w io.Writer) error {
				_, err := io.WriteString(w, "tar:"+image)
				return err
			},
		})
		var loaded string
		var ranSpec sandbox.RunSpec
		stubMicrosandbox(t, &fakeMicrosandbox{
			loadImage: func(r io.Reader, _ string) error {
				b, err := io.ReadAll(r)
				loaded = string(b)
				return err
			},
			run: func(rs sandbox.RunSpec) error {
				ranSpec = rs
				return nil
			},
		})
		stubFlag(t, runToolCmd, "runtime", "microsandbox")

		// Act
		err := runTool(runToolCmd, []string{"claude", "--help"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "tar:agentic-claude:latest", loaded)
		assert.Equal(t, "agentic-claude:abc123", ranSpec.Image)
		assert.Equal(t, []string{"--help"}, ranSpec.ToolArgs)
		rs := get()
		assert.Empty(t, rs.Image, "the tool should not run")
	})
}

func Test_parseArgs(t *testing.T) {
	t.Run("tool name and image name", func(t *testing.T) {
		// Act
		result, err := parseArgs([]string{"claude"}, "agentic")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "claude", result.toolName)
		assert.Equal(t, "agentic-claude", result.imageName)
		assert.Empty(t, result.toolArgs)
		assert.False(t, result.skipEntrypoint)
	})

	t.Run("tool args", func(t *testing.T) {
		// Act
		result, err := parseArgs([]string{"claude", "--flag", "value"}, "agentic")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--flag", "value"}, result.toolArgs)
		assert.False(t, result.skipEntrypoint)
	})

	t.Run("dash dash sets skip entrypoint", func(t *testing.T) {
		// Act
		result, err := parseArgs([]string{"claude", "--", "bash", "-c", "echo hi"}, "agentic")

		// Assert
		require.NoError(t, err)
		assert.True(t, result.skipEntrypoint)
		assert.Equal(t, []string{"bash", "-c", "echo hi"}, result.toolArgs)
	})

	t.Run("dash dash no trailing args", func(t *testing.T) {
		// Act
		result, err := parseArgs([]string{"claude", "--"}, "agentic")

		// Assert
		require.NoError(t, err)
		assert.True(t, result.skipEntrypoint)
		assert.Empty(t, result.toolArgs)
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		_, err := parseArgs([]string{"bogus"}, "agentic")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bogus")
	})
}

func Test_runRequest(t *testing.T) {
	// Arrange
	withTempToolHome(t)
	stubFlag(t, runToolCmd, "trust-dir", "true")
	stubFlag(t, runToolCmd, "dry-run", "true")
	stubFlag(t, runToolCmd, "volume", "/example.test/a:/a")
	stubFlag(t, runToolCmd, "secret", "token:/example.test/token")
	stubFlag(t, runToolCmd, "env", "FOO=bar")
	stubFlag(t, runToolCmd, "registry", "registry.example.test")
	stubFlag(t, runToolCmd, "proxy", "true")
	stubFlag(t, runToolCmd, "runtime", "microsandbox")
	inv := invocation{
		parsedArgs: parsedArgs{toolName: "claude", imageName: "agentic-claude", skipEntrypoint: true},
		cwd:        "/example.test/project",
		rc:         &config.AgenticRC{},
	}

	// Act
	req, err := runRequest(runToolCmd, inv)

	// Assert - run-only flags land next to the shared runtimeFlags
	require.NoError(t, err)
	assert.Equal(t, run.Target{ToolName: "claude", ImageName: "agentic-claude", SkipEntrypoint: true}, req.Target)
	assert.Equal(t, "/example.test/project", req.Project.Dir)
	assert.True(t, req.Flags.TrustDir)
	assert.True(t, req.Flags.DryRun)
	assert.True(t, req.Flags.Proxy.ProxyFlag)
	assert.Equal(t, "registry.example.test", req.Flags.Registry)
	assert.Equal(t, []string{"/example.test/a:/a"}, req.Flags.Volumes)
	assert.Equal(t, []string{"token:/example.test/token"}, req.Flags.Secrets)
	assert.Equal(t, []string{"FOO=bar"}, req.Flags.Env)
	assert.Equal(t, sandbox.RuntimeMicrosandbox, req.Flags.Runtime)
}
