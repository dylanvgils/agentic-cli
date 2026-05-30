package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddBuildFlags(t *testing.T) {
	t.Run("registers all flags", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}

		// Act
		addBuildFlags(cmd)

		// Assert
		for _, name := range []string{"base", "node", "java", "dotnet", "go", "apt", "dry-run"} {
			assert.NotNil(t, cmd.Flags().Lookup(name), "expected flag --%s to be registered", name)
		}
	})

	t.Run("version flag usage reflects default versions", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		addBuildFlags(cmd)

		cases := []struct {
			flag    string
			version string
		}{
			{"node", tools.DefaultVersions.Node},
			{"java", tools.DefaultVersions.Java},
			{"dotnet", tools.DefaultVersions.Dotnet},
			{"go", tools.DefaultVersions.Go},
		}

		// Assert
		for _, tc := range cases {
			f := cmd.Flags().Lookup(tc.flag)
			require.NotNil(t, f, "flag --%s not registered", tc.flag)
			assert.Contains(t, f.Usage, tc.version, "flag --%s usage should mention version %s", tc.flag, tc.version)
		}
	})

	t.Run("dry run flag defaults false", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}

		// Act
		addBuildFlags(cmd)

		// Assert
		f := cmd.Flags().Lookup("dry-run")
		require.NotNil(t, f)
		assert.Equal(t, "false", f.DefValue)
	})
}


func TestFlagOrEnv(t *testing.T) {
	t.Run("flag takes priority", func(t *testing.T) {
		// Arrange
		cmd := newFlagCmd(t, "node")
		require.NoError(t, cmd.Flags().Set("node", "fromflag"))
		t.Setenv("AGENTIC_NODE_VERSION", "fromenv")

		// Act
		result := flagOrEnv(cmd, "node", "AGENTIC_NODE_VERSION")

		// Assert
		assert.Equal(t, "fromflag", result)
	})

	t.Run("falls back to env when flag empty", func(t *testing.T) {
		// Arrange
		cmd := newFlagCmd(t, "node")
		t.Setenv("AGENTIC_NODE_VERSION", "fromenv")

		// Act
		result := flagOrEnv(cmd, "node", "AGENTIC_NODE_VERSION")

		// Assert
		assert.Equal(t, "fromenv", result)
	})

	t.Run("returns empty when both unset", func(t *testing.T) {
		// Arrange
		cmd := newFlagCmd(t, "node")

		// Act + Assert
		assert.Equal(t, "", flagOrEnv(cmd, "node", "AGENTIC_NODE_VERSION"))
	})
}

func newAptCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	addBuildFlags(cmd)
	return cmd
}

func TestCollectAptPackages(t *testing.T) {
	t.Run("rc file packages are included", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".agenticrc"), []byte("apt_packages=make\n"), 0o644))
		t.Chdir(dir)
		cmd := newAptCmd(t)

		// Act
		result := collectAptPackages(cmd)

		// Assert
		assert.Equal(t, []string{"make"}, result)
	})

	t.Run("flag appends to config packages", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".agenticrc"), []byte("apt_packages=make\n"), 0o644))
		t.Chdir(dir)
		cmd := newAptCmd(t)
		require.NoError(t, cmd.Flags().Set("apt", "gcc"))

		// Act
		result := collectAptPackages(cmd)

		// Assert
		assert.Equal(t, []string{"make", "gcc"}, result)
	})

	t.Run("empty when no sources set", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		cmd := newAptCmd(t)

		// Act
		result := collectAptPackages(cmd)

		// Assert
		assert.Empty(t, result)
	})
}

func TestBuildOptsFromFlags(t *testing.T) {
	t.Run("node env var used when flag absent", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_NODE_VERSION", "20")

		// Act
		opts := buildOptsFromFlags(buildCmd)

		// Assert
		assert.Equal(t, "20", opts.NodeVersion)
	})

	t.Run("version env vars used when flags absent", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_JAVA_VERSION", "17")
		t.Setenv("AGENTIC_DOTNET_VERSION", "8")
		t.Setenv("AGENTIC_GO_VERSION", "1.22")

		// Act
		opts := buildOptsFromFlags(buildCmd)

		// Assert
		assert.Equal(t, "17", opts.Versions["java"])
		assert.Equal(t, "8", opts.Versions["dotnet"])
		assert.Equal(t, "1.22", opts.Versions["go"])
	})

	t.Run("multiple base flags are joined", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		addBuildFlags(cmd)
		require.NoError(t, cmd.Flags().Set("base", "java"))
		require.NoError(t, cmd.Flags().Set("base", "dotnet"))

		// Act
		opts := buildOptsFromFlags(cmd)

		// Assert
		assert.Equal(t, "java,dotnet", opts.BaseOverride)
	})

	t.Run("base env var used when flag absent", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_BASE_OVERRIDE", "java")
		cmd := &cobra.Command{Use: "test"}
		addBuildFlags(cmd)

		// Act
		opts := buildOptsFromFlags(cmd)

		// Assert
		assert.Equal(t, "java", opts.BaseOverride)
	})
}

func TestToolNames(t *testing.T) {
	t.Run("no args returns all tools", func(t *testing.T) {
		// Act
		result := toolNames([]string{})

		// Assert
		assert.Equal(t, []string{"claude", "copilot", "opencode"}, result)
	})

	t.Run("single arg returns that tool", func(t *testing.T) {
		// Act
		result := toolNames([]string{"claude"})

		// Assert
		assert.Equal(t, []string{"claude"}, result)
	})
}

func TestPruneAndReport(t *testing.T) {
	t.Run("prints message when reclaimed non empty", func(t *testing.T) {
		// Arrange
		stubPruneImages(t, func() (string, error) { return "500MB", nil })

		// Act
		out := captureStdout(t, func() {
			err := pruneAndReport()
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "=> pruned dangling images (reclaimed 500MB)")
	})

	t.Run("silent when reclaimed empty", func(t *testing.T) {
		// Arrange
		stubPruneImages(t, func() (string, error) { return "", nil })

		// Act
		out := captureStdout(t, func() {
			err := pruneAndReport()
			require.NoError(t, err)
		})

		// Assert
		assert.NotContains(t, out, "pruned")
	})

	t.Run("propagates error", func(t *testing.T) {
		// Arrange
		stubPruneImages(t, func() (string, error) {
			return "", fmt.Errorf("prune failed")
		})

		// Act
		err := pruneAndReport()

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "prune failed")
	})
}
