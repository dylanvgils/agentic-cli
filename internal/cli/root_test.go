package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckDocker(t *testing.T) {
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

	t.Run("completion subcommand skips check", func(t *testing.T) {
		// Arrange - `agentic completion bash` reaches persistentPreRunE with cmd.Name()=="bash",
		// which is not in noDockerCmds; the ancestor walk must find "completion" instead.
		stubCheckDockerDaemon(t, func() error {
			return errors.New("should not be called")
		})
		fakeRoot := &cobra.Command{Use: "agentic"}
		completionCmd := &cobra.Command{Use: "completion"}
		bashCmd := &cobra.Command{Use: "bash"}
		fakeRoot.AddCommand(completionCmd)
		completionCmd.AddCommand(bashCmd)

		// Act
		err := checkDocker(bashCmd, nil)

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

	t.Run("marketplaces list subcommand skips check", func(t *testing.T) {
		// Arrange - ancestor walk must find "marketplaces", not just cmd.Name()=="list"
		stubCheckDockerDaemon(t, func() error {
			return errors.New("should not be called")
		})

		// Act
		err := checkDocker(marketplacesListCmd, nil)

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

	t.Run("no dry run flag calls check", func(t *testing.T) {
		// Arrange
		var called bool
		stubCheckDockerDaemon(t, func() error {
			called = true
			return nil
		})

		// Act
		err := checkDocker(inspectCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.True(t, called)
	})
}

func TestCheckGit(t *testing.T) {
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

func TestResolveContext(t *testing.T) {
	t.Run("passes the resolved flag value to setContext", func(t *testing.T) {
		// Arrange
		var got string
		stubSetContext(t, func(ctx string) { got = ctx })
		cmd := &cobra.Command{}
		cmd.Flags().String("docker-context", "", "")
		require.NoError(t, cmd.Flags().Set("docker-context", "prod"))

		// Act
		resolveContext(cmd)

		// Assert
		assert.Equal(t, "prod", got)
	})
}

func TestPersistentPreRunE(t *testing.T) {
	t.Run("resolves docker context before checking the daemon", func(t *testing.T) {
		// Arrange
		var got string
		stubSetContext(t, func(ctx string) { got = ctx })
		stubCheckDockerDaemon(t, func() error { return nil })
		cmd := &cobra.Command{Use: "status"}
		cmd.Flags().String("docker-context", "", "")
		require.NoError(t, cmd.Flags().Set("docker-context", "prod"))
		fakeRoot := &cobra.Command{Use: "agentic"}
		fakeRoot.AddCommand(cmd)

		// Act
		err := persistentPreRunE(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "prod", got)
	})
}

func TestInCommandChain(t *testing.T) {
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

func TestPruneResources(t *testing.T) {
	t.Run("calls pruneImages", func(t *testing.T) {
		// Arrange
		var called bool
		stubPruneImages(t, func() error { called = true; return nil })
		stubPruneBuildCache(t, func() error { return nil })

		// Act
		pruneResources()

		// Assert
		assert.True(t, called)
	})

	t.Run("calls pruneBuildCache", func(t *testing.T) {
		// Arrange
		var called bool
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { called = true; return nil })

		// Act
		pruneResources()

		// Assert
		assert.True(t, called)
	})

	t.Run("silent on error", func(t *testing.T) {
		// Arrange
		stubPruneImages(t, func() error { return fmt.Errorf("prune failed") })
		stubPruneBuildCache(t, func() error { return fmt.Errorf("cache prune failed") })

		// Act + Assert
		assert.NotPanics(t, pruneResources)
	})
}

func TestRootRun(t *testing.T) {
	t.Run("non-TTY falls back to help", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, func() bool { return false })
		stubRunDashboard(t, func() error {
			return errors.New("should not be called")
		})
		cmd := &cobra.Command{Use: "agentic"}
		cmd.SetOut(new(bytes.Buffer))

		// Act
		err := rootRun(cmd, nil)

		// Assert
		require.NoError(t, err)
	})

	t.Run("TTY launches the dashboard", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, func() bool { return true })
		var called bool
		stubRunDashboard(t, func() error { called = true; return nil })

		// Act
		err := rootRun(&cobra.Command{Use: "agentic"}, nil)

		// Assert
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("TTY propagates dashboard error", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, func() bool { return true })
		stubRunDashboard(t, func() error { return errors.New("dashboard failed") })

		// Act
		err := rootRun(&cobra.Command{Use: "agentic"}, nil)

		// Assert
		assert.ErrorContains(t, err, "dashboard failed")
	})
}
