package build

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStdout replaces os.Stdout with a pipe and returns what was written (e.g. DryRun's Dockerfile output); for logging.Step/Detail output, use captureLog.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stdout
	os.Stdout = w
	fn()
	w.Close() //nolint:errcheck
	os.Stdout = orig
	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck
	return buf.String()
}

// captureLog swaps logging.Log for the duration of fn and returns what was written.
func captureLog(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer
	orig := logging.Log
	logging.Log = logging.New(&buf)
	t.Cleanup(func() { logging.Log = orig })

	fn()

	return buf.String()
}

func TestDryRun(t *testing.T) {
	t.Run("prints dockerfile skips script", func(t *testing.T) {
		// Arrange
		var scriptCalled bool
		stubBuildTool(t, func(_, _ string, _ tools.BuildOptions) error {
			scriptCalled = true
			return nil
		})

		// Act
		out := captureStdout(t, func() {
			err := DryRun([]string{"claude"}, tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.False(t, scriptCalled)
		assert.Contains(t, out, "FROM")
		assert.NotContains(t, out, "proxy")
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		err := DryRun([]string{"nonexistent"}, tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown tool")
	})
}

func TestApply(t *testing.T) {
	t.Run("all tools when no args", func(t *testing.T) {
		// Arrange
		var built []string
		stubBuildTool(t, func(tool, _ string, _ tools.BuildOptions) error {
			built = append(built, tool)
			return nil
		})

		// Act
		err := Apply([]string{"claude", "copilot", "opencode"}, "agentic", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"claude", "copilot", "opencode"}, built)
	})

	t.Run("single tool when arg given", func(t *testing.T) {
		// Arrange
		var built []string
		stubBuildTool(t, func(tool, _ string, _ tools.BuildOptions) error {
			built = append(built, tool)
			return nil
		})

		// Act
		out := captureLog(t, func() {
			err := Apply([]string{"claude"}, "agentic", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, []string{"claude"}, built)
		assert.Contains(t, out, "=> agentic-claude")
		assert.NotContains(t, out, "=> copilot")
		assert.NotContains(t, out, "=> opencode")
	})

	t.Run("base override shown", func(t *testing.T) {
		// Arrange
		stubBuildTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		opts := tools.BuildOptions{BaseOverride: []string{"java"}, Versions: map[string]string{}}

		// Act
		out := captureLog(t, func() {
			err := Apply([]string{"claude"}, "agentic", opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "   base: java")
	})

	t.Run("base override with multiple extras shown", func(t *testing.T) {
		// Arrange
		stubBuildTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		opts := tools.BuildOptions{BaseOverride: []string{"java", "dotnet"}, Versions: map[string]string{}}

		// Act
		out := captureLog(t, func() {
			err := Apply([]string{"claude"}, "agentic", opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "   base: java, dotnet")
	})

	t.Run("apt packages shown", func(t *testing.T) {
		// Arrange
		stubBuildTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		opts := tools.BuildOptions{AptPackages: []string{"curl", "jq"}, Versions: map[string]string{}}

		// Act
		out := captureLog(t, func() {
			err := Apply([]string{"claude"}, "agentic", opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "   apt: curl, jq")
	})

	t.Run("apt packages hidden when empty", func(t *testing.T) {
		// Arrange
		stubBuildTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })

		// Act
		out := captureLog(t, func() {
			err := Apply([]string{"claude"}, "agentic", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.NotContains(t, out, "apt:")
	})

	t.Run("base override hidden when empty", func(t *testing.T) {
		// Arrange
		stubBuildTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })

		// Act
		out := captureLog(t, func() {
			err := Apply([]string{"claude"}, "agentic", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.NotContains(t, out, "=> base:")
	})

	t.Run("empty base-exact reported as none, exact", func(t *testing.T) {
		// Arrange
		stubBuildTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		opts := tools.BuildOptions{BaseExact: true, Versions: map[string]string{}}

		// Act
		out := captureLog(t, func() {
			err := Apply([]string{"claude"}, "agentic", opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "   base: (none, exact)")
	})

	t.Run("empty apt-exact reported as none, exact", func(t *testing.T) {
		// Arrange
		stubBuildTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		opts := tools.BuildOptions{AptExact: true, Versions: map[string]string{}}

		// Act
		out := captureLog(t, func() {
			err := Apply([]string{"claude"}, "agentic", opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "   apt: (none, exact)")
	})

	t.Run("script error propagates", func(t *testing.T) {
		// Arrange
		stubBuildTool(t, func(_, _ string, _ tools.BuildOptions) error {
			return fmt.Errorf("docker daemon not running")
		})

		// Act
		err := Apply([]string{"claude"}, "agentic", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		err := Apply([]string{"nonexistent"}, "agentic", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown tool")
	})

	t.Run("stops on first tool error", func(t *testing.T) {
		// Arrange
		var built []string
		stubBuildTool(t, func(tool, _ string, _ tools.BuildOptions) error {
			built = append(built, tool)
			return fmt.Errorf("fail on %s", tool)
		})

		// Act
		err := Apply([]string{"claude", "copilot", "opencode"}, "agentic", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
		assert.Len(t, built, 1)
	})
}
