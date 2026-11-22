package cli

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config/agenticrc"
	"github.com/dylanvgils/agentic-cli/internal/config/precedence"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/run"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_addBuildFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "--base and --base-exact are mutually exclusive", args: []string{"--base", "java", "--base-exact", "node"}},
		{name: "--apt and --apt-exact are mutually exclusive", args: []string{"--apt", "make", "--apt-exact", "gcc"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			cmd := &cobra.Command{Use: "test", RunE: func(*cobra.Command, []string) error { return nil }}
			addBuildFlags(cmd)
			cmd.SetArgs(tc.args)

			// Act
			err := cmd.Execute()

			// Assert
			assert.Error(t, err)
		})
	}

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

// Test_proxyInput only confirms each flag maps to the right precedence.ProxyInput field; precedence is covered by TestProxyMode in internal/config/precedence.
func Test_proxyInput(t *testing.T) {
	// Arrange
	cmd := &cobra.Command{Use: "test"}
	addProxyFlags(cmd)
	require.NoError(t, cmd.Flags().Set("no-proxy", "true"))
	require.NoError(t, cmd.Flags().Set("proxy-monitor", "true"))
	require.NoError(t, cmd.Flags().Set("proxy", "true"))
	require.NoError(t, cmd.Flags().Set("proxy-integration", "maven,gradle"))
	require.NoError(t, cmd.Flags().Set("proxy-backend", "agentic"))

	// Act
	result := proxyInput(cmd)

	// Assert
	assert.Equal(t, precedence.ProxyInput{
		NoProxy:      true,
		MonitorFlag:  true,
		ProxyFlag:    true,
		Integrations: []string{"maven", "gradle"},
		Backend:      "agentic",
	}, result)
}

// Test_dindInput only confirms each flag maps to the right precedence.DindInput field; precedence is covered by TestDindEnabled in internal/config/precedence.
func Test_dindInput(t *testing.T) {
	// Arrange
	cmd := &cobra.Command{Use: "test"}
	addDindFlags(cmd)
	require.NoError(t, cmd.Flags().Set("no-dind", "true"))

	// Act
	result := dindInput(cmd)

	// Assert
	assert.Equal(t, precedence.DindInput{NoDindFlag: true}, result)
}

func Test_buildOptsFromFlags(t *testing.T) {
	// Arrange
	rc := &agenticrc.Config{Build: agenticrc.Build{Bases: []string{"java"}}}
	cmd := &cobra.Command{Use: "test"}
	addBuildFlags(cmd)
	require.NoError(t, cmd.Flags().Set("base-exact", ""))

	// Act
	opts := buildOptsFromFlags(cmd, rc)

	// Assert - an empty --base-exact produces debian only
	assert.Empty(t, opts.BaseOverride)
	assert.True(t, opts.BaseExact)
}

func Test_requireArgOrAll(t *testing.T) {
	validate := requireArgOrAll("tool")

	for _, tc := range []struct {
		name    string
		args    []string
		all     bool
		wantErr string
	}{
		{name: "no arg and no all is refused", wantErr: "specify a tool or --all"},
		{name: "an arg passes", args: []string{"claude"}},
		{name: "all passes", all: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			cmd := newTestAllCmd(t, tc.all)

			// Act
			err := validate(cmd, tc.args)

			// Assert
			if tc.wantErr != "" {
				assert.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_argOrAllExclusive(t *testing.T) {
	for _, tc := range []struct {
		name    string
		all     bool
		wantErr string
	}{
		{name: "an arg with all is refused", all: true, wantErr: "an argument and --all can't be used together"},
		{name: "an arg alone passes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			cmd := newTestAllCmd(t, tc.all)

			// Act
			err := argOrAllExclusive(cmd, []string{"claude"})

			// Assert
			if tc.wantErr != "" {
				assert.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_warnSkipInstallChecksum(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opts    tools.BuildOptions
		wantLog string
	}{
		{
			name:    "warns when the checksum is skipped",
			opts:    tools.BuildOptions{SkipInstallChecksum: true},
			wantLog: "agentic: warning: --skip-install-checksum disables install script integrity verification\n",
		},
		{
			name: "stays quiet otherwise",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			logs := iostreamstest.StubErr(t)

			// Act
			warnSkipInstallChecksum(tc.opts)

			// Assert
			assert.Equal(t, tc.wantLog, logs.String())
		})
	}
}

func Test_runtimeFlags(t *testing.T) {
	withTempToolHome(t)

	newCmd := func() *cobra.Command {
		cmd := &cobra.Command{Use: "test"}
		addResourceLimitFlags(cmd)
		addProxyFlags(cmd)
		addDindFlags(cmd)
		addPortFlag(cmd)
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
		require.NoError(t, cmd.Flags().Set("port", "3000"))
		require.NoError(t, cmd.Flags().Set("runtime", "microsandbox"))

		// Act
		flags, err := runtimeFlags(cmd)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, run.Flags{
			ToolHome:   toolHome,
			Proxy:      precedence.ProxyInput{MonitorFlag: true, Integrations: []string{"maven"}},
			Dind:       precedence.DindInput{DindFlag: true},
			Limits:     sandbox.ResourceLimits{CPUs: "2"},
			DindLimits: sandbox.ResourceLimits{Memory: "1g"},
			Runtime:    sandbox.RuntimeMicrosandbox,
			Ports:      []string{"3000"},
		}, flags)
	})

	t.Run("unset runtime stays empty so the config can pick", func(t *testing.T) {
		// Act
		flags, err := runtimeFlags(newCmd())

		// Assert
		require.NoError(t, err)
		assert.Empty(t, flags.Runtime)
	})

	t.Run("unknown proxy backend returns error", func(t *testing.T) {
		// Arrange
		cmd := newCmd()
		require.NoError(t, cmd.Flags().Set("proxy-backend", "bogus"))

		// Act
		_, err := runtimeFlags(cmd)

		// Assert
		assert.EqualError(t, err, `--proxy-backend: unknown proxy backend "bogus": must be "native" or "agentic"`)
	})
}
