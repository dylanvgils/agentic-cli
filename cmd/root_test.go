package cmd

import (
	"errors"
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

		// Act — rootCmd.Parent() == nil satisfies the guard in checkDocker.
		err := checkDocker(rootCmd, nil)

		// Assert
		require.NoError(t, err)
	})

	t.Run("completion command skips check", func(t *testing.T) {
		// Arrange
		stubCheckDockerDaemon(t, func() error {
			return errors.New("should not be called")
		})
		cmd := &cobra.Command{Use: "completion"}

		// Act
		err := checkDocker(cmd, nil)

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
