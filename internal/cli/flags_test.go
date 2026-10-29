package cli

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/dylanvgils/agentic-cli/internal/usecase/run"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_addBuildFlags(t *testing.T) {

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

		// Act
		addBuildFlags(cmd)

		// Assert
		for _, name := range tools.KnownLayers() {
			f := cmd.Flags().Lookup(name)
			require.NotNil(t, f, "flag --%s not registered", name)
			assert.Contains(t, f.Usage, tools.DefaultVersions.ForLayer(name), "flag --%s usage should mention its default version", name)
		}
	})
}

func Test_addProxyFlags(t *testing.T) {
	// Arrange
	cmd := &cobra.Command{Use: "test", RunE: func(*cobra.Command, []string) error { return nil }}
	addProxyFlags(cmd)
	cmd.SetArgs([]string{"--proxy", "--no-proxy"})

	// Act
	err := cmd.Execute()

	// Assert
	assert.Error(t, err)
}

func Test_addDindFlags(t *testing.T) {
	// Arrange
	cmd := &cobra.Command{Use: "test", RunE: func(*cobra.Command, []string) error { return nil }}
	addDindFlags(cmd)
	cmd.SetArgs([]string{"--dind", "--no-dind"})

	// Act
	err := cmd.Execute()

	// Assert
	assert.Error(t, err)
}

// Test_proxyInput only confirms each flag maps to the right resolve.ProxyInput field; precedence is covered by TestProxyMode in internal/usecase/resolve.
func Test_proxyInput(t *testing.T) {
	t.Run("maps each flag", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		addProxyFlags(cmd)
		require.NoError(t, cmd.Flags().Set("no-proxy", "true"))
		require.NoError(t, cmd.Flags().Set("proxy-monitor", "true"))
		require.NoError(t, cmd.Flags().Set("proxy", "true"))
		require.NoError(t, cmd.Flags().Set("proxy-integration", "maven,gradle"))

		// Act
		result, err := proxyInput(cmd)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, resolve.ProxyInput{
			NoProxy:      true,
			MonitorFlag:  true,
			ProxyFlag:    true,
			Integrations: []string{"maven", "gradle"},
		}, result)
	})

	t.Run("unknown integration returns error", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		addProxyFlags(cmd)
		require.NoError(t, cmd.Flags().Set("proxy-integration", "bogus"))

		// Act
		_, err := proxyInput(cmd)

		// Assert
		assert.ErrorContains(t, err, `--proxy-integration: unknown integration "bogus"`)
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
	assert.Equal(t, sandbox.ResourceLimits{CPUs: "2", Memory: "8g"}, result)
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
	assert.Equal(t, sandbox.ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}, result)
}

func Test_buildOptsFromFlags(t *testing.T) {
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
}

func Test_toolNames(t *testing.T) {
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

func Test_warnSkipInstallChecksum(t *testing.T) {
	t.Run("warns when the checksum is skipped", func(t *testing.T) {
		// Arrange
		logs := stubErrLog(t)

		// Act
		warnSkipInstallChecksum(tools.BuildOptions{SkipInstallChecksum: true})

		// Assert
		assert.Contains(t, logs.String(), "--skip-install-checksum disables install script integrity verification")
	})

	t.Run("stays quiet otherwise", func(t *testing.T) {
		// Arrange
		logs := stubErrLog(t)

		// Act
		warnSkipInstallChecksum(tools.BuildOptions{})

		// Assert
		assert.Empty(t, logs.String())
	})
}

func Test_runtimeFlags(t *testing.T) {
	withTempToolHome(t)

	newCmd := func() *cobra.Command {
		cmd := &cobra.Command{Use: "test"}
		addResourceLimitFlags(cmd)
		addProxyFlags(cmd)
		addDindFlags(cmd)
		addRuntimeFlag(cmd)
		return cmd
	}

	t.Run("reads the shared flags", func(t *testing.T) {
		// Arrange
		cmd := newCmd()
		require.NoError(t, cmd.Flags().Set("proxy-monitor", "true"))
		require.NoError(t, cmd.Flags().Set("proxy-integration", "maven"))
		require.NoError(t, cmd.Flags().Set("dind", "true"))
		require.NoError(t, cmd.Flags().Set("cpus", "2"))
		require.NoError(t, cmd.Flags().Set("dind-memory", "1g"))
		require.NoError(t, cmd.Flags().Set("runtime", "microsandbox"))

		// Act
		flags, err := runtimeFlags(cmd)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, run.Flags{
			ToolHome:   toolHome,
			Proxy:      resolve.ProxyInput{MonitorFlag: true, Integrations: []string{"maven"}},
			Dind:       resolve.DindInput{DindFlag: true},
			Limits:     sandbox.ResourceLimits{CPUs: "2"},
			DindLimits: sandbox.ResourceLimits{Memory: "1g"},
			Runtime:    sandbox.RuntimeMicrosandbox,
		}, flags)
	})

	t.Run("unset runtime stays empty so the config can pick", func(t *testing.T) {
		// Act
		flags, err := runtimeFlags(newCmd())

		// Assert
		require.NoError(t, err)
		assert.Empty(t, flags.Runtime)
	})

	t.Run("unknown proxy integration returns error", func(t *testing.T) {
		// Arrange
		cmd := newCmd()
		require.NoError(t, cmd.Flags().Set("proxy-integration", "bogus"))

		// Act
		_, err := runtimeFlags(cmd)

		// Assert
		assert.ErrorContains(t, err, `unknown integration "bogus"`)
	})

	t.Run("unknown runtime returns error", func(t *testing.T) {
		// Arrange
		cmd := newCmd()
		require.NoError(t, cmd.Flags().Set("runtime", "podman"))

		// Act
		_, err := runtimeFlags(cmd)

		// Assert
		assert.ErrorContains(t, err, `unknown runtime "podman"`)
	})
}
