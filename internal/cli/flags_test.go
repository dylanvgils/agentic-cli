package cli

import (
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

func TestAddResourceLimitFlags(t *testing.T) {
	t.Run("registers all flags", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}

		// Act
		addResourceLimitFlags(cmd)

		// Assert
		for _, name := range []string{"pids-limit", "cpus", "memory"} {
			assert.NotNil(t, cmd.Flags().Lookup(name), "expected flag --%s to be registered", name)
		}
	})
}

func TestAddProxyFlags(t *testing.T) {
	t.Run("registers all flags", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}

		// Act
		addProxyFlags(cmd)

		// Assert
		for _, name := range []string{"proxy", "no-proxy", "proxy-monitor"} {
			assert.NotNil(t, cmd.Flags().Lookup(name), "expected flag --%s to be registered", name)
		}
	})

	t.Run("proxy flags are mutually exclusive", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test", RunE: func(*cobra.Command, []string) error { return nil }}
		addProxyFlags(cmd)
		cmd.SetArgs([]string{"--proxy", "--no-proxy"})

		// Act
		err := cmd.Execute()

		// Assert
		assert.Error(t, err)
	})
}

func newBuildCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	addBuildFlags(cmd)
	return cmd
}

// TestCollectBases, TestCollectVersions, and TestCollectAptPackages only need
// to confirm the flag value is read and passed through - the merge/precedence
// behavior itself is covered by internal/usecase/settings, which these
// delegate to.
func TestCollectBases(t *testing.T) {
	t.Run("flag value is read and merged via settings.Bases", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}}}
		cmd := newBuildCmd(t)
		require.NoError(t, cmd.Flags().Set("base", "dotnet"))

		// Act
		result := collectBases(cmd, rc)

		// Assert - sorted by canonical extras order
		assert.Equal(t, []string{"dotnet", "java"}, result)
	})
}

func TestCollectVersions(t *testing.T) {
	t.Run("flag value is read and merged via settings.Versions", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{Versions: map[string]string{"java": "17"}}}
		cmd := newBuildCmd(t)
		require.NoError(t, cmd.Flags().Set("java", "21"))

		// Act
		result := collectVersions(cmd, rc)

		// Assert
		assert.Equal(t, "21", result["java"])
	})
}

func TestCollectAptPackages(t *testing.T) {
	t.Run("flag value is read and merged via settings.AptPackages", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{AptPackages: []string{"make"}}}
		cmd := newBuildCmd(t)
		require.NoError(t, cmd.Flags().Set("apt", "gcc"))

		// Act
		result := collectAptPackages(cmd, rc)

		// Assert
		assert.Equal(t, []string{"make", "gcc"}, result)
	})
}

func TestBuildOptsFromFlags(t *testing.T) {
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
