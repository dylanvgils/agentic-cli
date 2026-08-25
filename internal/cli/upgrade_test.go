package cli

import (
	"errors"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stubFetchLatestVersion(t *testing.T, v string, err error) {
	t.Helper()
	orig := fetchLatestVersion
	fetchLatestVersion = func() (string, error) { return v, err }
	t.Cleanup(func() { fetchLatestVersion = orig })
}

func stubPerformUpdate(t *testing.T, err error) {
	t.Helper()
	orig := performUpdate
	performUpdate = func(_ string) error { return err }
	t.Cleanup(func() { performUpdate = orig })
}

func TestRunUpgrade(t *testing.T) {
	t.Run("prints already up to date when no newer version", func(t *testing.T) {
		// Arrange
		stubFetchLatestVersion(t, "v1.0.0", nil)
		stubPerformUpdate(t, nil)
		origVersion := buildinfo.Version
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = origVersion })

		// Act
		out := captureLog(t, func() {
			err := runUpgrade(upgradeCmd, nil)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "already up to date")
		assert.Contains(t, out, "v1.0.0")
	})

	t.Run("calls performUpdate when newer version available", func(t *testing.T) {
		// Arrange
		var updateCalledWith string
		stubFetchLatestVersion(t, "v1.1.0", nil)
		orig := performUpdate
		performUpdate = func(v string) error {
			updateCalledWith = v
			return nil
		}
		t.Cleanup(func() { performUpdate = orig })
		origVersion := buildinfo.Version
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = origVersion })

		// Act
		out := captureLog(t, func() {
			err := runUpgrade(upgradeCmd, nil)
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, "v1.1.0", updateCalledWith)
		assert.Contains(t, out, "updated to v1.1.0")
	})

	t.Run("returns error when fetch fails", func(t *testing.T) {
		// Arrange
		stubFetchLatestVersion(t, "", errors.New("network error"))
		origVersion := buildinfo.Version
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = origVersion })

		// Act
		err := runUpgrade(upgradeCmd, nil)

		// Assert
		assert.Error(t, err)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		// Arrange
		stubFetchLatestVersion(t, "v1.1.0", nil)
		stubPerformUpdate(t, errors.New("permission denied"))
		origVersion := buildinfo.Version
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = origVersion })

		// Act
		err := runUpgrade(upgradeCmd, nil)

		// Assert
		assert.Error(t, err)
	})

	t.Run("force skips up-to-date check", func(t *testing.T) {
		// Arrange
		upgradeForce = true
		t.Cleanup(func() { upgradeForce = false })
		var updateCalledWith string
		stubFetchLatestVersion(t, "v1.0.0", nil)
		orig := performUpdate
		performUpdate = func(v string) error { updateCalledWith = v; return nil }
		t.Cleanup(func() { performUpdate = orig })
		origVersion := buildinfo.Version
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = origVersion })

		// Act
		out := captureLog(t, func() {
			err := runUpgrade(upgradeCmd, nil)
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, "v1.0.0", updateCalledWith)
		assert.Contains(t, out, "updating")
	})

	t.Run("force skips pre-release check", func(t *testing.T) {
		// Arrange
		upgradeForce = true
		t.Cleanup(func() { upgradeForce = false })
		var updateCalledWith string
		stubFetchLatestVersion(t, "v1.0.0", nil)
		orig := performUpdate
		performUpdate = func(v string) error { updateCalledWith = v; return nil }
		t.Cleanup(func() { performUpdate = orig })
		origVersion := buildinfo.Version
		buildinfo.Version = "v1.0.0-alpha.1"
		t.Cleanup(func() { buildinfo.Version = origVersion })

		// Act
		err := runUpgrade(upgradeCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "v1.0.0", updateCalledWith)
	})

	t.Run("version flag installs specified version without fetching latest", func(t *testing.T) {
		// Arrange
		upgradeVersion = "v0.9.0"
		t.Cleanup(func() { upgradeVersion = "" })
		var fetchCalled bool
		orig := fetchLatestVersion
		fetchLatestVersion = func() (string, error) { fetchCalled = true; return "v1.0.0", nil }
		t.Cleanup(func() { fetchLatestVersion = orig })
		var updateCalledWith string
		origUpdate := performUpdate
		performUpdate = func(v string) error { updateCalledWith = v; return nil }
		t.Cleanup(func() { performUpdate = origUpdate })
		origVersion := buildinfo.Version
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = origVersion })

		// Act
		out := captureLog(t, func() {
			err := runUpgrade(upgradeCmd, nil)
			require.NoError(t, err)
		})

		// Assert
		assert.False(t, fetchCalled)
		assert.Equal(t, "v0.9.0", updateCalledWith)
		assert.Contains(t, out, "v0.9.0")
	})

	t.Run("version flag skips up-to-date check", func(t *testing.T) {
		// Arrange
		upgradeVersion = "v1.0.0"
		t.Cleanup(func() { upgradeVersion = "" })
		stubFetchLatestVersion(t, "v1.0.0", nil)
		var updateCalledWith string
		orig := performUpdate
		performUpdate = func(v string) error { updateCalledWith = v; return nil }
		t.Cleanup(func() { performUpdate = orig })
		origVersion := buildinfo.Version
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = origVersion })

		// Act
		err := runUpgrade(upgradeCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "v1.0.0", updateCalledWith)
	})
}
