package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_checkDocker(t *testing.T) {
	// childOf returns a command named use under a fake root, so it isn't the root itself.
	childOf := func(use string) *cobra.Command {
		cmd := &cobra.Command{Use: use}
		(&cobra.Command{Use: "agentic"}).AddCommand(cmd)
		return cmd
	}

	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "root command", cmd: rootCmd},
		{name: "completion command", cmd: childOf("completion")},
		{name: "shell completion", cmd: childOf(cobra.ShellCompRequestCmd)},
		{name: "aliases command", cmd: aliasesCmd},
		// migrate only touches the filesystem under TOOL_HOME, no Docker needed
		{name: "migrate command", cmd: migrateCmd},
	} {
		t.Run(tc.name+" skips check", func(t *testing.T) {
			// Arrange
			stubCheckDockerDaemon(t, func() error {
				return errors.New("should not be called")
			})

			// Act
			err := checkDocker(tc.cmd, nil)

			// Assert
			require.NoError(t, err)
		})
	}

	t.Run("calls check success", func(t *testing.T) {
		// Arrange
		var called bool
		stubCheckDockerDaemon(t, func() error {
			called = true
			return nil
		})

		// Act
		err := checkDocker(buildCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("calls check error", func(t *testing.T) {
		// Arrange
		stubCheckDockerDaemon(t, func() error {
			return docker.ErrDaemonNotRunning
		})

		// Act
		err := checkDocker(buildCmd, nil)

		// Assert
		assert.Equal(t, docker.ErrDaemonNotRunning, err)
	})
}

func Test_checkGit(t *testing.T) {
	runCmd := &cobra.Command{Use: "run"}
	// No git on PATH, so every check that runs fails
	t.Setenv("PATH", t.TempDir())

	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{name: "non-run command", cmd: buildCmd, args: []string{"claude"}},
		{name: "no args", cmd: runCmd},
		{name: "unknown tool", cmd: runCmd, args: []string{"bogus"}},
		// opencode has no MarketplaceMount support; toolNeedsMarketplaceSync covers the rest of this predicate's branches directly
		{name: "tool that doesn't need marketplace sync", cmd: runCmd, args: []string{"opencode"}},
	} {
		t.Run(tc.name+" skips check", func(t *testing.T) {
			// Arrange
			t.Chdir(t.TempDir())

			// Act
			err := checkGit(tc.cmd, tc.args)

			// Assert
			require.NoError(t, err)
		})
	}

	t.Run("marketplace-capable tool with marketplaces configured calls check", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rcContent := "[[marketplaces]]\nname = \"acme\"\nurl = \"git@example.com:acme.git\"\n"
		writeRC(t, dir, rcContent)
		t.Chdir(dir)

		// Act
		err := checkGit(runCmd, []string{"claude"})

		// Assert
		assert.ErrorContains(t, err, "git not found on PATH")
	})
}

func Test_resolveContext(t *testing.T) {
	t.Run("flag sets the context", func(t *testing.T) {
		// Arrange - keep the repo's own .agenticrc.toml out of the test
		t.Chdir(t.TempDir())
		restoreDockerClient(t)
		cmd := &cobra.Command{}
		cmd.Flags().String("docker-context", "", "")
		require.NoError(t, cmd.Flags().Set("docker-context", "prod"))

		// Act
		err := resolveContext(cmd)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "prod", dockerClient.Context())
	})

	t.Run("context from an untrusted workspace config is refused", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		writeRC(t, dir, `docker_context = "prod"`)
		t.Chdir(dir)
		restoreDockerClient(t)
		stubToolHome(t, t.TempDir())
		cmd := &cobra.Command{}
		cmd.Flags().String("docker-context", "", "")

		// Act
		err := resolveContext(cmd)

		// Assert
		assert.ErrorContains(t, err, "is not trusted")
	})
}

func Test_persistentPreRunE(t *testing.T) {
	// Keep the repo's own .agenticrc.toml out of the test
	t.Chdir(t.TempDir())

	// newCmd returns a command named use under a fake root, with the root's --docker-context flag.
	newCmd := func(t *testing.T, use string) *cobra.Command {
		t.Helper()
		restoreDockerClient(t)
		cmd := &cobra.Command{Use: use}
		cmd.Flags().String("docker-context", "", "")
		(&cobra.Command{Use: "agentic"}).AddCommand(cmd)
		return cmd
	}

	t.Run("resolves the docker context and loads the project onto the command's context", func(t *testing.T) {
		// Arrange
		stubToolHome(t, filepath.Join(t.TempDir(), "home"))
		cmd := newCmd(t, "status")
		require.NoError(t, cmd.Flags().Set("docker-context", "prod"))

		// Act
		err := persistentPreRunE(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "prod", dockerClient.Context())
		_, ok := cmd.Context().Value(projectKey{}).(loadedProject)
		assert.True(t, ok)
	})

	t.Run("migration failure aborts the command", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(home, ".migrations.json"), []byte("not json"), 0o644))
		stubToolHome(t, home)
		cmd := newCmd(t, "status")

		// Act
		err := persistentPreRunE(cmd, nil)

		// Assert
		assert.ErrorContains(t, err, "invalid character")
	})

	for _, tc := range []struct {
		name string
		use  string
	}{
		{name: "an excluded command", use: "upgrade"},
		{name: "shell completion", use: cobra.ShellCompRequestCmd},
	} {
		t.Run(tc.name+" skips the migration check", func(t *testing.T) {
			// Arrange
			home := filepath.Join(t.TempDir(), "home")
			stubToolHome(t, home)
			cmd := newCmd(t, tc.use)

			// Act
			err := persistentPreRunE(cmd, nil)

			// Assert
			require.NoError(t, err)
			assert.NoDirExists(t, home)
		})
	}
}

func Test_restartAfterUpgrade(t *testing.T) {
	t.Run("not updated does nothing", func(t *testing.T) {
		// Act
		err := restartAfterUpgrade(false, nil)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("failed upgrade returns its error", func(t *testing.T) {
		// Arrange
		upgradeErr := errors.New("update failed")

		// Act
		err := restartAfterUpgrade(false, upgradeErr)

		// Assert
		assert.ErrorIs(t, err, upgradeErr)
	})
}

func Test_inCommandChain(t *testing.T) {
	t.Run("matches command name", func(t *testing.T) {
		// Act
		result := inCommandChain(aliasesCmd, noUpdateCmds)

		// Assert
		assert.True(t, result)
	})

	t.Run("matches ancestor name for nested subcommand", func(t *testing.T) {
		// Arrange - `agentic completion zsh` reaches the update-check guard with
		// cmd.Name()=="zsh", which is not in noUpdateCmds; the ancestor walk must
		// find "completion" instead, since shells source this at startup.
		fakeRoot := &cobra.Command{Use: "agentic"}
		completionCmd := &cobra.Command{Use: "completion"}
		zshCmd := &cobra.Command{Use: "zsh"}
		fakeRoot.AddCommand(completionCmd)
		completionCmd.AddCommand(zshCmd)

		// Act
		result := inCommandChain(zshCmd, noUpdateCmds)

		// Assert
		assert.True(t, result)
	})

	t.Run("returns false when no ancestor matches", func(t *testing.T) {
		// Act
		result := inCommandChain(buildCmd, noUpdateCmds)

		// Assert
		assert.False(t, result)
	})
}

// TestHomeFlag checks --home stays one root flag that no command shadows with its own.
func TestHomeFlag(t *testing.T) {
	// Arrange
	var own []string
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, sub := range cmd.Commands() {
			if sub.LocalNonPersistentFlags().Lookup("home") != nil || sub.PersistentFlags().Lookup("home") != nil {
				own = append(own, sub.CommandPath())
			}
			walk(sub)
		}
	}

	// Act
	walk(rootCmd)

	// Assert
	require.NotNil(t, rootCmd.PersistentFlags().Lookup("home"))
	assert.Empty(t, own)
}

func Test_failUnknownHelpTopic(t *testing.T) {
	newRoot := func() *cobra.Command {
		root := &cobra.Command{Use: "app", SilenceErrors: true, SilenceUsage: true}
		root.AddCommand(&cobra.Command{Use: "sub", Short: "a subcommand", Run: func(*cobra.Command, []string) {}})
		root.SetOut(iostreams.Default().Out)
		root.SetErr(iostreams.Default().Err)
		failUnknownHelpTopic(root)
		return root
	}

	t.Run("unknown topic fails without output", func(t *testing.T) {
		// Arrange
		out := iostreamstest.StubCombined(t)
		root := newRoot()
		root.SetArgs([]string{"help", "nosuch"})

		// Act
		err := root.Execute()

		// Assert
		require.EqualError(t, err, `unknown help topic "nosuch"`)
		assert.Empty(t, out.String())
	})

	t.Run("known topic prints its help", func(t *testing.T) {
		// Arrange
		out := iostreamstest.StubOut(t)
		root := newRoot()
		root.SetArgs([]string{"help", "sub"})

		// Act
		err := root.Execute()

		// Assert
		require.NoError(t, err)
		assert.Contains(t, out.String(), "a subcommand")
	})
}
