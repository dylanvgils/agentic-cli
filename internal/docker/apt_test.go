package docker

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_verifyAptPackages(t *testing.T) {
	client := newTestClient()

	t.Run("empty packages skips docker call", func(t *testing.T) {
		// Arrange
		called := false
		stubRunInteractiveFunc(t, client, func(_ ...string) error {
			called = true
			return nil
		})

		// Act
		err := client.verifyAptPackages(nil, "")

		// Assert
		require.NoError(t, err)
		assert.False(t, called, "expected no docker call for empty package list")
	})

	t.Run("pulls image before checking packages", func(t *testing.T) {
		// Arrange
		get := stubRunInteractive(t, client)
		stubDockerRunFixed(t, client, "", nil)

		// Act
		err := client.verifyAptPackages([]string{"make"}, "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "pull", get()[0])
		assert.Contains(t, get(), "debian:"+tools.DefaultVersions.Debian)
	})

	t.Run("pulls registry-prefixed image when registry set", func(t *testing.T) {
		// Arrange
		get := stubRunInteractive(t, client)
		stubDockerRunFixed(t, client, "", nil)

		// Act
		err := client.verifyAptPackages([]string{"make"}, "myregistry.example.com")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, get(), "myregistry.example.com/debian:"+tools.DefaultVersions.Debian)
	})

	t.Run("returns specific error for missing packages", func(t *testing.T) {
		// Arrange
		stubRunInteractive(t, client)
		stubDockerRunFixed(t, client, "badpkg\n", nil)

		// Act
		err := client.verifyAptPackages([]string{"make", "badpkg"}, "")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "apt packages not found")
		assert.Contains(t, err.Error(), "badpkg")
		assert.NotContains(t, err.Error(), "make")
	})

	t.Run("pull error returns error", func(t *testing.T) {
		// Arrange
		stubRunInteractiveFunc(t, client, func(_ ...string) error { return fmt.Errorf("pull failed") })

		// Act
		err := client.verifyAptPackages([]string{"make"}, "")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to pull verification image")
	})
}

func Test_missingAptPackages(t *testing.T) {
	client := newTestClient()

	t.Run("passes packages as arguments", func(t *testing.T) {
		// Arrange
		var capturedArgs []string
		stubDockerRun(t, client, func(args ...string) (string, error) {
			capturedArgs = args
			return "", nil
		})

		// Act
		_, err := client.missingAptPackages([]string{"make", "gcc"}, "debian:bookworm-slim")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, capturedArgs, "debian:bookworm-slim")
		assert.Contains(t, capturedArgs, "make")
		assert.Contains(t, capturedArgs, "gcc")
	})

	t.Run("uses the provided image name", func(t *testing.T) {
		// Arrange
		var capturedArgs []string
		stubDockerRun(t, client, func(args ...string) (string, error) {
			capturedArgs = args
			return "", nil
		})

		// Act
		_, err := client.missingAptPackages([]string{"make"}, "myregistry.example.com/debian:bookworm-slim")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, capturedArgs, "myregistry.example.com/debian:bookworm-slim")
	})

	t.Run("returns missing package names from output", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "curl\nbadpkg\n", nil)

		// Act
		missing, err := client.missingAptPackages([]string{"make", "curl", "badpkg"}, "debian:bookworm-slim")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"curl", "badpkg"}, missing)
	})

	t.Run("returns empty for all packages found", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", nil)

		// Act
		missing, err := client.missingAptPackages([]string{"make", "gcc"}, "debian:bookworm-slim")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, missing)
	})

	t.Run("docker error returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", fmt.Errorf("exit status 1"))

		// Act
		missing, err := client.missingAptPackages([]string{"make"}, "debian:bookworm-slim")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "apt package verification failed")
		assert.Nil(t, missing)
	})
}
