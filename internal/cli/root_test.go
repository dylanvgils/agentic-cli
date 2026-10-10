package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/migrate"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_checkDocker(t *testing.T) {
	t.Run("root command skips check", func(t *testing.T) {
		// Arrange
		stubCheckDockerDaemon(t, func() error {
			return errors.New("should not be called")
		})

		// Act - rootCmd.Parent() == nil satisfies the guard in checkDocker.
		err := checkDocker(rootCmd, nil)

		// Assert
		require.NoError(t, err)
	})

	t.Run("completion command skips check", func(t *testing.T) {
		// Arrange
		stubCheckDockerDaemon(t, func() error {
			return errors.New("should not be called")
		})
		fakeRoot := &cobra.Command{Use: "agentic"}
		completionCmd := &cobra.Command{Use: "completion"}
		fakeRoot.AddCommand(completionCmd)

		// Act
		err := checkDocker(completionCmd, nil)

		// Assert
		require.NoError(t, err)
	})

	t.Run("shell completion skips check", func(t *testing.T) {
		// Arrange
		stubCheckDockerDaemon(t, func() error {
			return errors.New("should not be called")
		})
		fakeRoot := &cobra.Command{Use: "agentic"}
		completeCmd := &cobra.Command{Use: cobra.ShellCompRequestCmd}
		fakeRoot.AddCommand(completeCmd)

		// Act
		err := checkDocker(completeCmd, nil)

		// Assert
		require.NoError(t, err)
	})

	t.Run("aliases command skips check", func(t *testing.T) {
		// Arrange
		stubCheckDockerDaemon(t, func() error {
			return errors.New("should not be called")
		})

		// Act
		err := checkDocker(aliasesCmd, nil)

		// Assert
		require.NoError(t, err)
	})

	t.Run("migrate command skips check", func(t *testing.T) {
		// Arrange - migrate only touches the filesystem under TOOL_HOME, no Docker needed
		stubCheckDockerDaemon(t, func() error {
			return errors.New("should not be called")
		})

		// Act
		err := checkDocker(migrateCmd, nil)

		// Assert
		require.NoError(t, err)
	})

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

	t.Run("non-run command skips check", func(t *testing.T) {
		// Arrange
		stubCheckGitAvailable(t, errors.New("should not be called"))

		// Act
		err := checkGit(buildCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
	})

	t.Run("no args skips check", func(t *testing.T) {
		// Arrange
		stubCheckGitAvailable(t, errors.New("should not be called"))

		// Act
		err := checkGit(runCmd, nil)

		// Assert
		require.NoError(t, err)
	})

	t.Run("unknown tool skips check", func(t *testing.T) {
		// Arrange
		stubCheckGitAvailable(t, errors.New("should not be called"))

		// Act
		err := checkGit(runCmd, []string{"bogus"})

		// Assert
		require.NoError(t, err)
	})

	t.Run("tool that doesn't need marketplace sync skips check", func(t *testing.T) {
		// Arrange - opencode has no MarketplaceMount support; toolNeedsMarketplaceSync
		// covers the rest of this predicate's branches directly.
		t.Chdir(t.TempDir())
		stubCheckGitAvailable(t, errors.New("should not be called"))

		// Act
		err := checkGit(runCmd, []string{"opencode"})

		// Assert
		require.NoError(t, err)
	})

	t.Run("marketplace-capable tool with marketplaces configured calls check", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rcContent := "[[marketplaces]]\nname = \"acme\"\nurl = \"git@example.com:acme.git\"\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".agenticrc.toml"), []byte(rcContent), 0o644))
		t.Chdir(dir)
		stubCheckGitAvailable(t, errors.New("git not found on PATH"))

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
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".agenticrc.toml"), []byte(`docker_context = "prod"`), 0o644))
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

	t.Run("resolves the docker context", func(t *testing.T) {
		// Arrange
		restoreDockerClient(t)
		stubMigrateRun(t, func(string) ([]migrate.Migration, error) { return nil, nil })
		cmd := &cobra.Command{Use: "status"}
		cmd.Flags().String("docker-context", "", "")
		require.NoError(t, cmd.Flags().Set("docker-context", "prod"))
		fakeRoot := &cobra.Command{Use: "agentic"}
		fakeRoot.AddCommand(cmd)

		// Act
		err := persistentPreRunE(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "prod", dockerClient.Context())
	})

	t.Run("migration failure aborts the command", func(t *testing.T) {
		// Arrange
		restoreDockerClient(t)
		stubMigrateRun(t, func(string) ([]migrate.Migration, error) {
			return nil, errors.New("migration failed")
		})
		cmd := &cobra.Command{Use: "status"}
		cmd.Flags().String("docker-context", "", "")
		fakeRoot := &cobra.Command{Use: "agentic"}
		fakeRoot.AddCommand(cmd)

		// Act
		err := persistentPreRunE(cmd, nil)

		// Assert
		assert.ErrorContains(t, err, "migration failed")
	})

	t.Run("skips the migration check for excluded commands", func(t *testing.T) {
		// Arrange
		restoreDockerClient(t)
		stubMigrateRun(t, func(string) ([]migrate.Migration, error) {
			return nil, errors.New("should not be called")
		})
		cmd := &cobra.Command{Use: "upgrade"}
		cmd.Flags().String("docker-context", "", "")
		fakeRoot := &cobra.Command{Use: "agentic"}
		fakeRoot.AddCommand(cmd)

		// Act
		err := persistentPreRunE(cmd, nil)

		// Assert
		require.NoError(t, err)
	})

	t.Run("shell completion skips the migration check", func(t *testing.T) {
		// Arrange
		restoreDockerClient(t)
		stubMigrateRun(t, func(string) ([]migrate.Migration, error) {
			return nil, errors.New("should not be called")
		})
		cmd := &cobra.Command{Use: cobra.ShellCompRequestCmd}
		cmd.Flags().String("docker-context", "", "")
		fakeRoot := &cobra.Command{Use: "agentic"}
		fakeRoot.AddCommand(cmd)

		// Act
		err := persistentPreRunE(cmd, nil)

		// Assert
		require.NoError(t, err)
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
