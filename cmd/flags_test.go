package cmd

import (
	"fmt"
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

func TestCollectAptPackages(t *testing.T) {
	t.Run("env var parsed as comma-separated packages", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_APT_PACKAGES", "make,gcc")
		cmd := newFlagCmd(t, "apt")

		// Act
		result := collectAptPackages(cmd)

		// Assert
		assert.Equal(t, []string{"make", "gcc"}, result)
	})

	t.Run("flag takes priority and appends to env", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_APT_PACKAGES", "make")
		cmd := newFlagCmd(t, "apt")
		require.NoError(t, cmd.Flags().Set("apt", "gcc"))

		// Act
		result := collectAptPackages(cmd)

		// Assert
		assert.Equal(t, []string{"make", "gcc"}, result)
	})

	t.Run("deduplicates across sources", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_APT_PACKAGES", "make")
		cmd := newFlagCmd(t, "apt")
		require.NoError(t, cmd.Flags().Set("apt", "make"))

		// Act
		result := collectAptPackages(cmd)

		// Assert
		count := 0
		for _, pkg := range result {
			if pkg == "make" {
				count++
			}
		}
		assert.Equal(t, 1, count, "make should appear exactly once")
	})

	t.Run("empty when no sources set", func(t *testing.T) {
		// Arrange
		cmd := newFlagCmd(t, "apt")

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
