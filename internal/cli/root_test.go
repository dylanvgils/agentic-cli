package cli

import (
	"errors"
	"fmt"
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
