package cmd

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddAllFlag(t *testing.T) {
	t.Run("registers -a shorthand", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}

		// Act
		addAllFlag(cmd)

		// Assert
		assert.NotNil(t, cmd.Flags().ShorthandLookup("a"))
	})
}

func TestAddNamespaceFlag(t *testing.T) {
	t.Run("registers -n shorthand", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}

		// Act
		addNamespaceFlag(cmd)

		// Assert
		assert.NotNil(t, cmd.Flags().ShorthandLookup("n"))
	})
}

func TestAddBuildFlags(t *testing.T) {
	t.Run("registers all flags", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}

		// Act
		addBuildFlags(cmd)

		// Assert
		expected := append([]string{"base", "apt", "dry-run", "registry"}, tools.KnownLayers()...)
		for _, name := range expected {
			assert.NotNil(t, cmd.Flags().Lookup(name), "expected flag --%s to be registered", name)
		}
	})

	t.Run("version flag usage reflects default versions", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		addBuildFlags(cmd)

		var cases []struct {
			flag    string
			version string
		}
		for _, name := range tools.KnownLayers() {
			cases = append(cases, struct {
				flag    string
				version string
			}{name, tools.DefaultVersions.ForLayer(name)})
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

func newBuildCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	addBuildFlags(cmd)
	return cmd
}

func newAptCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	addBuildFlags(cmd)
	return cmd
}

func TestCollectAptPackages(t *testing.T) {
	t.Run("rc packages are included", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{AptPackages: []string{"make"}}}
		cmd := newAptCmd(t)

		// Act
		result := collectAptPackages(cmd, rc)

		// Assert
		assert.Equal(t, []string{"make"}, result)
	})

	t.Run("flag appends to config packages", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{AptPackages: []string{"make"}}}
		cmd := newAptCmd(t)
		require.NoError(t, cmd.Flags().Set("apt", "gcc"))

		// Act
		result := collectAptPackages(cmd, rc)

		// Assert
		assert.Equal(t, []string{"make", "gcc"}, result)
	})

	t.Run("empty when no sources set", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		cmd := newAptCmd(t)

		// Act
		result := collectAptPackages(cmd, rc)

		// Assert
		assert.Empty(t, result)
	})
}

func TestCollectBases(t *testing.T) {
	t.Run("rc bases are included", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}}}
		cmd := newBuildCmd(t)

		// Act
		result := collectBases(cmd, rc)

		// Assert
		assert.Equal(t, []string{"java"}, result)
	})

	t.Run("flag appends to rc bases", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}}}
		cmd := newBuildCmd(t)
		require.NoError(t, cmd.Flags().Set("base", "dotnet"))

		// Act
		result := collectBases(cmd, rc)

		// Assert - sorted by canonical extras order
		assert.Equal(t, []string{"dotnet", "java"}, result)
	})

	t.Run("empty when no sources set", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		cmd := newBuildCmd(t)

		// Act
		result := collectBases(cmd, rc)

		// Assert
		assert.Empty(t, result)
	})
}

func TestCollectVersions(t *testing.T) {
	t.Run("rc versions used as defaults", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{Versions: map[string]string{"java": "17"}}}
		cmd := newBuildCmd(t)

		// Act
		result := collectVersions(cmd, rc)

		// Assert
		assert.Equal(t, "17", result["java"])
	})

	t.Run("flag overrides rc version", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{Versions: map[string]string{"java": "17"}}}
		cmd := newBuildCmd(t)
		require.NoError(t, cmd.Flags().Set("java", "21"))

		// Act
		result := collectVersions(cmd, rc)

		// Assert
		assert.Equal(t, "21", result["java"])
	})

	t.Run("empty when no sources set", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		cmd := newBuildCmd(t)

		// Act
		result := collectVersions(cmd, rc)

		// Assert
		assert.Empty(t, result)
	})
}

func TestBuildOptsFromFlags(t *testing.T) {
	t.Run("multiple base flags are joined", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		cmd := &cobra.Command{Use: "test"}
		addBuildFlags(cmd)
		require.NoError(t, cmd.Flags().Set("base", "java"))
		require.NoError(t, cmd.Flags().Set("base", "dotnet"))

		// Act
		opts := buildOptsFromFlags(cmd, rc)

		// Assert
		assert.Equal(t, []string{"dotnet", "java"}, opts.BaseOverride)
	})

	t.Run("base env var overrides rc and flag", func(t *testing.T) {
		// Arrange
		t.Setenv(config.EnvBaseOverride, "dotnet")
		rc := &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}}}
		cmd := &cobra.Command{Use: "test"}
		addBuildFlags(cmd)
		require.NoError(t, cmd.Flags().Set("base", "go"))

		// Act
		opts := buildOptsFromFlags(cmd, rc)

		// Assert
		assert.Equal(t, []string{"dotnet"}, opts.BaseOverride)
	})
}

func TestExtrasEnvDoc(t *testing.T) {
	// Act
	result := extrasEnvDoc()

	// Assert
	for _, name := range tools.KnownLayers() {
		assert.Contains(t, result, config.EnvVersionVar(name), "env doc missing var for layer %q", name)
	}
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
