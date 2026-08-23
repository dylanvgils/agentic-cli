package upgradecheck

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/logging"
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

		// Act
		Check(home)

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

func Test_notifyUpdate(t *testing.T) {
	origVersion := buildinfo.Version
	buildinfo.Version = "v1.0.0"
	t.Cleanup(func() { buildinfo.Version = origVersion })

	t.Run("prints one-liner to stderr when not a terminal", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, false)
		errBuf := stubStderrCapture(t)

		// Act
		notifyUpdate("v1.1.0")

		// Assert
		out := errBuf.String()
		assert.Contains(t, out, "v1.1.0")
		assert.Contains(t, out, "v1.0.0")
		assert.Contains(t, out, "upgrade")
	})

	t.Run("prompts and updates when terminal and user confirms", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)
		updateCalledWith := stubUpdateCapture(t, nil)

		origStdin := Stdin
		Stdin = strings.NewReader("y\n")
		t.Cleanup(func() { Stdin = origStdin })

		errBuf := stubStderrCapture(t)
		exitCode := stubExitCapture(t)

		// Act
		notifyUpdate("v1.1.0")

		// Assert
		assert.Equal(t, "v1.1.0", *updateCalledWith)
		assert.Contains(t, errBuf.String(), "updated to v1.1.0")
		assert.Equal(t, 0, *exitCode)
	})

	t.Run("exits with code 1 when terminal, user confirms, and update fails", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)
		stubUpdate(t, errors.New("network error"))

		origStdin := Stdin
		Stdin = strings.NewReader("y\n")
		t.Cleanup(func() { Stdin = origStdin })

		errBuf := stubStderrCapture(t)
		exitCode := stubExitCapture(t)

		// Act
		notifyUpdate("v1.1.0")

		// Assert
		assert.Contains(t, errBuf.String(), "update failed")
		assert.Equal(t, 1, *exitCode)
	})

	t.Run("skips update when terminal and user declines", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)
		updateCalled := stubUpdateCapture(t, nil)

		origStdin := Stdin
		Stdin = strings.NewReader("n\n")
		t.Cleanup(func() { Stdin = origStdin })

		origNotify := Notify
		Notify = logging.New(io.Discard)
		t.Cleanup(func() { Notify = origNotify })

		// Act
		notifyUpdate("v1.1.0")

		// Assert
		assert.Empty(t, *updateCalled)
	})
}
