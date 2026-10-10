package upgradecheck

import (
	"errors"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheck(t *testing.T) {
	t.Run("skips when version is dev", func(t *testing.T) {
		// Arrange
		stubBuildVersion(t, "dev")
		fetchCalled := stubLatestVersion(t, "v1.1.0", nil)
		home := t.TempDir()
		confirm := func(string, string) bool { return true }

		// Act
		Check(home, confirm)

		// Assert
		assert.False(t, *fetchCalled)
	})

	t.Run("offers a due newer release", func(t *testing.T) {
		// Arrange
		stubBuildVersion(t, "v1.0.0")
		stubLatestVersion(t, "v1.1.0", nil)
		stubUpdate(t, nil)
		stubNotify(t)
		stubExit(t)
		home := t.TempDir()
		var offered string
		confirm := func(_, latest string) bool {
			offered = latest
			return true
		}

		// Act
		Check(home, confirm)

		// Assert
		assert.Equal(t, "v1.1.0", offered)
	})
}

func Test_fetchUpdateIfDue(t *testing.T) {
	stubBuildVersion(t, "v1.0.0")

	t.Run("returns false when within check interval", func(t *testing.T) {
		// Arrange
		fetchCalled := stubLatestVersion(t, "v1.1.0", nil)
		home := t.TempDir()
		lastCheck := time.Now().Add(-1 * time.Hour)
		cfg := &config.State{LastUpdateCheck: &lastCheck}
		require.NoError(t, cfg.Save(home))

		// Act
		latest, ok := fetchUpdateIfDue(home)

		// Assert
		assert.False(t, *fetchCalled)
		assert.False(t, ok)
		assert.Empty(t, latest)
	})

	t.Run("returns false when fetch fails", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "", errors.New("network error"))
		home := t.TempDir()

		// Act
		latest, ok := fetchUpdateIfDue(home)

		// Assert
		assert.False(t, ok)
		assert.Empty(t, latest)
	})

	t.Run("does not save LastUpdateCheck when fetch fails", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "", errors.New("network error"))
		home := t.TempDir()

		// Act
		_, ok := fetchUpdateIfDue(home)

		// Assert
		require.False(t, ok)
		cfg, err := config.LoadState(home)
		require.NoError(t, err)
		assert.Nil(t, cfg.LastUpdateCheck)
	})

	t.Run("returns false when already up to date", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "v1.0.0", nil)
		home := t.TempDir()

		// Act
		latest, ok := fetchUpdateIfDue(home)

		// Assert
		assert.False(t, ok)
		assert.Empty(t, latest)
	})

	t.Run("saves LastUpdateCheck after fetch", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "v1.0.0", nil)
		home := t.TempDir()
		before := time.Now()

		// Act
		_, ok := fetchUpdateIfDue(home)

		// Assert
		require.False(t, ok)
		cfg, err := config.LoadState(home)
		require.NoError(t, err)
		require.NotNil(t, cfg.LastUpdateCheck)
		assert.True(t, cfg.LastUpdateCheck.After(before))
	})

	t.Run("returns latest and true when update available", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "v1.1.0", nil)
		home := t.TempDir()

		// Act
		latest, ok := fetchUpdateIfDue(home)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, "v1.1.0", latest)
	})
}

func Test_offerUpdate(t *testing.T) {
	stubBuildVersion(t, "v1.0.0")

	t.Run("asks confirm with installed and latest", func(t *testing.T) {
		// Arrange
		var installed, latest string
		confirm := func(i, l string) bool {
			installed, latest = i, l
			return false
		}

		// Act
		offerUpdate("v1.1.0", confirm)

		// Assert
		assert.Equal(t, "v1.0.0", installed)
		assert.Equal(t, "v1.1.0", latest)
	})

	t.Run("updates and exits 0 when confirmed", func(t *testing.T) {
		// Arrange
		updateCalledWith := stubUpdate(t, nil)
		errBuf := stubNotify(t)
		exitCode := stubExit(t)
		confirm := func(string, string) bool { return true }

		// Act
		offerUpdate("v1.1.0", confirm)

		// Assert
		assert.Equal(t, "v1.1.0", *updateCalledWith)
		assert.Contains(t, errBuf.String(), "updated to v1.1.0")
		assert.Equal(t, 0, *exitCode)
	})

	t.Run("exits 1 when confirmed and the update fails", func(t *testing.T) {
		// Arrange
		stubUpdate(t, errors.New("network error"))
		errBuf := stubNotify(t)
		exitCode := stubExit(t)
		confirm := func(string, string) bool { return true }

		// Act
		offerUpdate("v1.1.0", confirm)

		// Assert
		assert.Contains(t, errBuf.String(), "update failed")
		assert.Equal(t, 1, *exitCode)
	})

	t.Run("skips update when declined", func(t *testing.T) {
		// Arrange
		updateCalledWith := stubUpdate(t, nil)
		exitCode := stubExit(t)
		confirm := func(string, string) bool { return false }

		// Act
		offerUpdate("v1.1.0", confirm)

		// Assert
		assert.Empty(t, *updateCalledWith)
		assert.Equal(t, -1, *exitCode)
	})
}
