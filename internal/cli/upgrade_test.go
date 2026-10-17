package cli

import (
	"errors"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runUpgrade(t *testing.T) {
	origVersion := buildinfo.Version
	buildinfo.Version = "v1.0.0"
	t.Cleanup(func() { buildinfo.Version = origVersion })

	t.Run("prints already up to date when no newer version", func(t *testing.T) {
		// Arrange
		releases := &fakeReleases{latest: "v1.0.0"}
		stubReleases(t, releases)

		// Act
		out := captureLog(t, func() {
			err := runUpgrade(upgradeCmd, nil)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "already up to date")
		assert.Contains(t, out, "v1.0.0")
		assert.Empty(t, releases.updatedWith)
	})

	t.Run("updates when a newer version is available", func(t *testing.T) {
		// Arrange
		releases := &fakeReleases{latest: "v1.1.0"}
		stubReleases(t, releases)

		// Act
		out := captureLog(t, func() {
			err := runUpgrade(upgradeCmd, nil)
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, "v1.1.0", releases.updatedWith)
		assert.Contains(t, out, "updated to v1.1.0")
	})

	t.Run("returns error when fetch fails", func(t *testing.T) {
		// Arrange
		stubReleases(t, &fakeReleases{latestErr: errors.New("network error")})

		// Act
		err := runUpgrade(upgradeCmd, nil)

		// Assert
		assert.ErrorContains(t, err, "network error")
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		// Arrange
		stubReleases(t, &fakeReleases{latest: "v1.1.0", updateErr: errors.New("permission denied")})

		// Act
		err := runUpgrade(upgradeCmd, nil)

		// Assert
		assert.ErrorContains(t, err, "permission denied")
	})

	t.Run("force skips up-to-date check", func(t *testing.T) {
		// Arrange
		upgradeForce = true
		t.Cleanup(func() { upgradeForce = false })
		releases := &fakeReleases{latest: "v1.0.0"}
		stubReleases(t, releases)
		logBuf := stubErrLog(t)

		// Act
		err := runUpgrade(upgradeCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "v1.0.0", releases.updatedWith)
		assert.Contains(t, logBuf.String(), "updating")
	})

	t.Run("version flag installs specified version without fetching latest", func(t *testing.T) {
		// Arrange
		upgradeVersion = "v0.9.0"
		t.Cleanup(func() { upgradeVersion = "" })
		releases := &fakeReleases{latest: "v1.0.0"}
		stubReleases(t, releases)

		// Act
		out := captureLog(t, func() {
			err := runUpgrade(upgradeCmd, nil)
			require.NoError(t, err)
		})

		// Assert
		assert.False(t, releases.fetched)
		assert.Equal(t, "v0.9.0", releases.updatedWith)
		assert.Contains(t, out, "v0.9.0")
	})
}
