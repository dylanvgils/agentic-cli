package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_verifyAptPackages(t *testing.T) {
	t.Run("empty packages skips docker call", func(t *testing.T) {
		// Arrange
		called := false
		orig := runInteractive
		runInteractive = func(_ ...string) error {
			called = true
			return nil
		}
		t.Cleanup(func() { runInteractive = orig })

		// Act
		err := verifyAptPackages(nil, "")

		// Assert
		require.NoError(t, err)
		assert.False(t, called, "expected no docker call for empty package list")
	})

	t.Run("pulls image before checking packages", func(t *testing.T) {
		// Arrange
		get := stubRunInteractive(t)
		stubDockerRunFixed(t, "", nil)

		// Act
		err := verifyAptPackages([]string{"make"}, "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "pull", get()[0])
		assert.Contains(t, get(), "debian:bookworm-slim")
	})

	t.Run("pulls registry-prefixed image when registry set", func(t *testing.T) {
		// Arrange
		get := stubRunInteractive(t)
		stubDockerRunFixed(t, "", nil)

		// Act
		err := verifyAptPackages([]string{"make"}, "myregistry.example.com")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, get(), "myregistry.example.com/debian:bookworm-slim")
	})

	t.Run("returns specific error for missing packages", func(t *testing.T) {
		// Arrange
		stubRunInteractive(t)
		stubDockerRunFixed(t, "badpkg\n", nil)

		// Act
		err := verifyAptPackages([]string{"make", "badpkg"}, "")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "apt packages not found")
		assert.Contains(t, err.Error(), "badpkg")
		assert.NotContains(t, err.Error(), "make")
	})

	t.Run("pull error returns error", func(t *testing.T) {
		// Arrange
		orig := runInteractive
		runInteractive = func(_ ...string) error { return fmt.Errorf("pull failed") }
		t.Cleanup(func() { runInteractive = orig })

		// Act
		err := verifyAptPackages([]string{"make"}, "")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to pull verification image")
	})
}

func Test_missingAptPackages(t *testing.T) {
	t.Run("passes packages as arguments", func(t *testing.T) {
		// Arrange
		var capturedArgs []string
		stubDockerRun(t, func(args ...string) (string, error) {
			capturedArgs = args
			return "", nil
		})

		// Act
		_, err := missingAptPackages([]string{"make", "gcc"}, "debian:bookworm-slim")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, capturedArgs, "debian:bookworm-slim")
		assert.Contains(t, capturedArgs, "make")
		assert.Contains(t, capturedArgs, "gcc")
	})

	t.Run("uses the provided image name", func(t *testing.T) {
		// Arrange
		var capturedArgs []string
		stubDockerRun(t, func(args ...string) (string, error) {
			capturedArgs = args
			return "", nil
		})

		// Act
		_, err := missingAptPackages([]string{"make"}, "myregistry.example.com/debian:bookworm-slim")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, capturedArgs, "myregistry.example.com/debian:bookworm-slim")
	})

	t.Run("returns missing package names from output", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "curl\nbadpkg\n", nil)

		// Act
		missing, err := missingAptPackages([]string{"make", "curl", "badpkg"}, "debian:bookworm-slim")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"curl", "badpkg"}, missing)
	})

	t.Run("returns empty for all packages found", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", nil)

		// Act
		missing, err := missingAptPackages([]string{"make", "gcc"}, "debian:bookworm-slim")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, missing)
	})

	t.Run("docker error returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", fmt.Errorf("exit status 1"))

		// Act
		missing, err := missingAptPackages([]string{"make"}, "debian:bookworm-slim")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "apt package verification failed")
		assert.Nil(t, missing)
	})
}
