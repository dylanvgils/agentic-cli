package cli

import (
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runInstructions(t *testing.T) {
	// Arrange
	stubDocker(t, &fakeDocker{})
	t.Chdir(t.TempDir())

	// Act
	var err error
	out := iostreamstest.CaptureOut(t, func() {
		err = runInstructions(instructionsCmd, []string{"claude"})
	})

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out, "## Installed toolchains")
	assert.Contains(t, out, "## Filesystem")
}

func Test_instructionsRequest(t *testing.T) {
	t.Run("request targets the tool's image in the resolved namespace with the runtime flags", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		stubFlag(t, instructionsCmd, "runtime", "microsandbox")

		// Act
		req, namespace, err := instructionsRequest(instructionsCmd, "claude")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "claude", req.Target.ToolName)
		assert.Equal(t, namespace+"-claude", req.Target.ImageName)
		assert.Equal(t, sandbox.RuntimeMicrosandbox, req.Flags.Runtime)
	})

	for _, tc := range []struct {
		name    string
		tool    string
		runtime string
		wantErr string
	}{
		{name: "unknown tool", tool: "bogus", wantErr: "bogus"},
		{name: "unknown runtime", tool: "claude", runtime: "podman", wantErr: `unknown runtime "podman"`},
	} {
		t.Run(tc.name+" returns error", func(t *testing.T) {
			// Arrange
			t.Chdir(t.TempDir())
			if tc.runtime != "" {
				stubFlag(t, instructionsCmd, "runtime", tc.runtime)
			}

			// Act
			_, _, err := instructionsRequest(instructionsCmd, tc.tool)

			// Assert
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}

	t.Run("invalid project config fails fast with a clear error", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		writeRC(t, dir, "not valid toml [[[")
		rcPath := filepath.Join(dir, ".agenticrc.toml")
		t.Chdir(dir)

		// Act
		_, _, err := instructionsRequest(instructionsCmd, "claude")

		// Assert
		assert.ErrorContains(t, err, rcPath)
	})
}

func Test_printInstructions(t *testing.T) {
	t.Run("content is printed under its heading", func(t *testing.T) {
		// Act
		out := iostreamstest.CaptureOut(t, func() { printInstructions("agentic/claude", "## Filesystem\n") })

		// Assert
		assert.Equal(t, "=> agentic/claude\n## Filesystem\n", out)
	})

	t.Run("empty content prints the disabled notice", func(t *testing.T) {
		// Act
		logs := iostreamstest.CaptureOut(t, func() { printInstructions("agentic/claude", "") })

		// Assert
		assert.Contains(t, logs, "disabled via .agenticrc.toml")
	})
}
