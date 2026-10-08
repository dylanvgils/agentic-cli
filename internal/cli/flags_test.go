package cli

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
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
		expected := append([]string{"base", "base-exact", "apt", "apt-exact", "dry-run", "registry"}, tools.KnownLayers()...)
		for _, name := range expected {
			assert.NotNil(t, cmd.Flags().Lookup(name), "expected flag --%s to be registered", name)
		}
	})

	t.Run("base and base-exact are mutually exclusive", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test", RunE: func(*cobra.Command, []string) error { return nil }}
		addBuildFlags(cmd)
		cmd.SetArgs([]string{"--base", "java", "--base-exact", "node"})

		// Act
		err := cmd.Execute()

		// Assert
		assert.Error(t, err)
	})

	t.Run("apt and apt-exact are mutually exclusive", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test", RunE: func(*cobra.Command, []string) error { return nil }}
		addBuildFlags(cmd)
		cmd.SetArgs([]string{"--apt", "make", "--apt-exact", "gcc"})

		// Act
		err := cmd.Execute()

		// Assert
		assert.Error(t, err)
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

func Test_addDindFlags(t *testing.T) {
	t.Run("registers toggle and limit flags", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}

		// Act
		addDindFlags(cmd)

		// Assert
		for _, name := range []string{"dind", "no-dind", "dind-pids-limit", "dind-cpus", "dind-memory"} {
			assert.NotNil(t, cmd.Flags().Lookup(name), "expected flag --%s to be registered", name)
		}
	})

	t.Run("dind flags are mutually exclusive", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test", RunE: func(*cobra.Command, []string) error { return nil }}
		addDindFlags(cmd)
		cmd.SetArgs([]string{"--dind", "--no-dind"})

		// Act
		err := cmd.Execute()

		// Assert
		assert.Error(t, err)
	})
}

// Test_dindInput only confirms each flag maps to the right resolve.DindInput field; precedence is covered by TestDindEnabled in internal/usecase/resolve.
func Test_dindInput(t *testing.T) {
	t.Run("dind flag maps to DindFlag", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		addDindFlags(cmd)
		require.NoError(t, cmd.Flags().Set("dind", "true"))

		// Act
		result := dindInput(cmd)

		// Assert
		assert.Equal(t, resolve.DindInput{DindFlag: true}, result)
	})

	t.Run("no-dind flag maps to NoDindFlag", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		addDindFlags(cmd)
		require.NoError(t, cmd.Flags().Set("no-dind", "true"))

		// Act
		result := dindInput(cmd)

		// Assert
		assert.Equal(t, resolve.DindInput{NoDindFlag: true}, result)
	})
}

func Test_resolveDindResourceLimitFlags(t *testing.T) {
	// Arrange
	cmd := &cobra.Command{Use: "test"}
	addDindFlags(cmd)
	require.NoError(t, cmd.Flags().Set("dind-cpus", "2"))
	require.NoError(t, cmd.Flags().Set("dind-memory", "8g"))

	// Act
	result := resolveDindResourceLimitFlags(cmd)

	// Assert
	assert.Equal(t, docker.ResourceLimits{CPUs: "2", Memory: "8g"}, result)
}

func Test_resolveResourceLimitFlags(t *testing.T) {
	// Arrange
	cmd := &cobra.Command{Use: "test"}
	addResourceLimitFlags(cmd)
	require.NoError(t, cmd.Flags().Set("pids-limit", "512"))
	require.NoError(t, cmd.Flags().Set("cpus", "2"))
	require.NoError(t, cmd.Flags().Set("memory", "2g"))

	// Act
	result := resolveResourceLimitFlags(cmd)

	// Assert
	assert.Equal(t, docker.ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}, result)
}

func TestBuildOptsFromFlags(t *testing.T) {
	t.Run("base flag values accumulate and merge with rc", func(t *testing.T) {
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

	t.Run("base-exact ignores rc bases", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}}}
		cmd := &cobra.Command{Use: "test"}
		addBuildFlags(cmd)
		require.NoError(t, cmd.Flags().Set("base-exact", "node"))

		// Act
		opts := buildOptsFromFlags(cmd, rc)

		// Assert
		assert.Equal(t, []string{"node"}, opts.BaseOverride)
		assert.True(t, opts.BaseExact)
	})

	t.Run("base-exact empty value produces debian only", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}}}
		cmd := &cobra.Command{Use: "test"}
		addBuildFlags(cmd)
		require.NoError(t, cmd.Flags().Set("base-exact", ""))

		// Act
		opts := buildOptsFromFlags(cmd, rc)

		// Assert
		assert.Empty(t, opts.BaseOverride)
		assert.True(t, opts.BaseExact)
	})

	t.Run("apt-exact ignores rc apt", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Build: config.RCBuild{AptPackages: []string{"make"}}}
		cmd := &cobra.Command{Use: "test"}
		addBuildFlags(cmd)
		require.NoError(t, cmd.Flags().Set("apt-exact", "gcc"))

		// Act
		opts := buildOptsFromFlags(cmd, rc)

		// Assert
		assert.Equal(t, []string{"gcc"}, opts.AptPackages)
		assert.True(t, opts.AptExact)
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

func Test_firstArg(t *testing.T) {
	t.Run("no args returns empty", func(t *testing.T) {
		// Act
		result := firstArg(nil)

		// Assert
		assert.Empty(t, result)
	})

	t.Run("returns the first arg", func(t *testing.T) {
		// Act
		result := firstArg([]string{"claude", "extra"})

		// Assert
		assert.Equal(t, "claude", result)
	})
}
