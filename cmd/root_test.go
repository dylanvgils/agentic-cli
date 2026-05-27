package cmd

import (
	"errors"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stubCheckDockerDaemon(t *testing.T, fn func() error) func() {
	t.Helper()
	orig := checkDockerDaemon
	checkDockerDaemon = fn
	return func() { checkDockerDaemon = orig }
}

// checkDocker — root command (no parent) skips daemon check (bare `agentic` shows help).
func TestCheckDocker_rootCommand_skipsCheck(t *testing.T) {
	// Arrange
	restore := stubCheckDockerDaemon(t, func() error {
		return errors.New("should not be called")
	})
	defer restore()

	// Act — rootCmd.Parent() == nil satisfies the guard in checkDocker.
	err := checkDocker(rootCmd, nil)

	// Assert
	require.NoError(t, err)
}

// checkDocker — `completion` subcommand skips daemon check.
func TestCheckDocker_completionCommand_skipsCheck(t *testing.T) {
	// Arrange
	restore := stubCheckDockerDaemon(t, func() error {
		return errors.New("should not be called")
	})
	defer restore()

	cmd := &cobra.Command{Use: "completion"}

	// Act
	err := checkDocker(cmd, nil)

	// Assert
	require.NoError(t, err)
}

// checkDocker — `aliases` subcommand skips daemon check (handles failure gracefully).
func TestCheckDocker_aliasesCommand_skipsCheck(t *testing.T) {
	// Arrange
	restore := stubCheckDockerDaemon(t, func() error {
		return errors.New("should not be called")
	})
	defer restore()

	// Act
	err := checkDocker(aliasesCmd, nil)

	// Assert
	require.NoError(t, err)
}

// checkDocker — calls daemon check (success path).
func TestCheckDocker_callsCheck_success(t *testing.T) {
	// Arrange
	var called bool
	restore := stubCheckDockerDaemon(t, func() error {
		called = true
		return nil
	})
	defer restore()

	// Act
	err := checkDocker(buildCmd, nil)

	// Assert
	require.NoError(t, err)
	assert.True(t, called)
}

// checkDocker — propagates daemon error.
func TestCheckDocker_callsCheck_error(t *testing.T) {
	// Arrange
	restore := stubCheckDockerDaemon(t, func() error {
		return docker.ErrDaemonNotRunning
	})
	defer restore()

	// Act
	err := checkDocker(buildCmd, nil)

	// Assert
	assert.Equal(t, docker.ErrDaemonNotRunning, err)
}

// checkDocker — command with no dry-run flag calls daemon check.
func TestCheckDocker_noDryRunFlag_callsCheck(t *testing.T) {
	// Arrange
	var called bool
	restore := stubCheckDockerDaemon(t, func() error {
		called = true
		return nil
	})
	defer restore()

	// Act
	err := checkDocker(inspectCmd, nil)

	// Assert
	require.NoError(t, err)
	assert.True(t, called)
}
