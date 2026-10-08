package upgradecheck

import (
	"errors"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheck(t *testing.T) {
	t.Run("skips when version is dev", func(t *testing.T) {
		// Arrange
		var fetchCalled bool
		orig := LatestVersion
		LatestVersion = func() (string, error) {
			fetchCalled = true
			return "v1.1.0", nil
		}
		t.Cleanup(func() { LatestVersion = orig })
		home := t.TempDir()
		confirm := func(string, string) bool { return true }

		// Act
		Check(home, confirm)

		// Assert
		assert.False(t, fetchCalled)
	})
}

func Test_fetchUpdateIfDue(t *testing.T) {
	t.Run("returns false when within check interval", func(t *testing.T) {
		// Arrange
		var fetchCalled bool
		orig := LatestVersion
		LatestVersion = func() (string, error) {
			fetchCalled = true
			return "v1.1.0", nil
		}
		t.Cleanup(func() { LatestVersion = orig })

		home := t.TempDir()
		lastCheck := time.Now().Add(-1 * time.Hour)
		cfg := &config.CliConfig{LastUpdateCheck: &lastCheck}
		require.NoError(t, cfg.Save(home))

		origVersion := buildinfo.Version
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = origVersion })

		// Act
		latest, ok := fetchUpdateIfDue(home)

		// Assert
		assert.False(t, fetchCalled)
		assert.False(t, ok)
		assert.Empty(t, latest)
	})

	t.Run("returns false when fetch fails", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "", errors.New("network error"))
		home := t.TempDir()
		origVersion := buildinfo.Version
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = origVersion })

		// Act
		latest, ok := fetchUpdateIfDue(home)

		// Assert
		assert.False(t, ok)
		assert.Empty(t, latest)
	})

	t.Run("returns false when already up to date", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "v1.0.0", nil)
		home := t.TempDir()
		origVersion := buildinfo.Version
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = origVersion })

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
		origVersion := buildinfo.Version
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = origVersion })
		before := time.Now()

		// Act
		fetchUpdateIfDue(home)

		// Assert
		cfg, err := config.LoadConfig(home)
		require.NoError(t, err)
		assert.NotNil(t, cfg.LastUpdateCheck)
		assert.True(t, cfg.LastUpdateCheck.After(before))
	})

	t.Run("returns latest and true when update available", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "v1.1.0", nil)
		home := t.TempDir()
		origVersion := buildinfo.Version
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = origVersion })

		// Act
		latest, ok := fetchUpdateIfDue(home)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, "v1.1.0", latest)
	})
}

func Test_offerUpdate(t *testing.T) {
	origVersion := buildinfo.Version
	buildinfo.Version = "v1.0.0"
	t.Cleanup(func() { buildinfo.Version = origVersion })

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
		updateCalledWith := stubUpdateCapture(t, nil)
		errBuf := stubStderrCapture(t)
		exitCode := stubExitCapture(t)
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
		errBuf := stubStderrCapture(t)
		exitCode := stubExitCapture(t)
		confirm := func(string, string) bool { return true }

		// Act
		offerUpdate("v1.1.0", confirm)

		// Assert
		assert.Contains(t, errBuf.String(), "update failed")
		assert.Equal(t, 1, *exitCode)
	})

	t.Run("skips update when declined", func(t *testing.T) {
		// Arrange
		updateCalled := stubUpdateCapture(t, nil)
		exited := false
		origExit := Exit
		Exit = func(int) { exited = true }
		t.Cleanup(func() { Exit = origExit })
		confirm := func(string, string) bool { return false }

		// Act
		offerUpdate("v1.1.0", confirm)

		// Assert
		assert.Empty(t, *updateCalled)
		assert.False(t, exited)
	})
}
