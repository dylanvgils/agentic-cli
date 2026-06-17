package cmd

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/config"
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

func stubIsTerminal(t *testing.T, terminal bool) {
	t.Helper()
	orig := isTerminal
	isTerminal = func() bool { return terminal }
	t.Cleanup(func() { isTerminal = orig })
}

func TestRunUpgrade(t *testing.T) {
	t.Run("prints already up to date when no newer version", func(t *testing.T) {
		// Arrange
		stubFetchLatestVersion(t, "v1.0.0", nil)
		stubPerformUpdate(t, nil)
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = "dev" })

		// Act
		out := captureStdout(t, func() {
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
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = "dev" })

		// Act
		out := captureStdout(t, func() {
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
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = "dev" })

		// Act
		err := runUpgrade(upgradeCmd, nil)

		// Assert
		assert.Error(t, err)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		// Arrange
		stubFetchLatestVersion(t, "v1.1.0", nil)
		stubPerformUpdate(t, errors.New("permission denied"))
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = "dev" })

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
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = "dev" })

		// Act
		out := captureStdout(t, func() {
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
		buildinfo.Version = "v1.0.0-alpha.1"
		t.Cleanup(func() { buildinfo.Version = "dev" })

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
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = "dev" })

		// Act
		out := captureStdout(t, func() {
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
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = "dev" })

		// Act
		err := runUpgrade(upgradeCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "v1.0.0", updateCalledWith)
	})
}

func TestMaybeNotifyUpdate(t *testing.T) {
	t.Run("skips when version is dev", func(t *testing.T) {
		// Arrange
		var fetchCalled bool
		orig := fetchLatestVersion
		fetchLatestVersion = func() (string, error) {
			fetchCalled = true
			return "v1.1.0", nil
		}
		t.Cleanup(func() { fetchLatestVersion = orig })
		home := t.TempDir()

		// Act
		maybeNotifyUpdate(home)

		// Assert
		assert.False(t, fetchCalled)
	})
}

func Test_fetchUpdateIfDue(t *testing.T) {
	t.Run("returns false when within check interval", func(t *testing.T) {
		// Arrange
		var fetchCalled bool
		orig := fetchLatestVersion
		fetchLatestVersion = func() (string, error) {
			fetchCalled = true
			return "v1.1.0", nil
		}
		t.Cleanup(func() { fetchLatestVersion = orig })

		home := t.TempDir()
		lastCheck := time.Now().Add(-1 * time.Hour)
		cfg := &config.CliConfig{LastUpdateCheck: &lastCheck}
		require.NoError(t, cfg.Save(home))

		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = "dev" })

		// Act
		latest, ok := fetchUpdateIfDue(home)

		// Assert
		assert.False(t, fetchCalled)
		assert.False(t, ok)
		assert.Empty(t, latest)
	})

	t.Run("returns false when fetch fails", func(t *testing.T) {
		// Arrange
		stubFetchLatestVersion(t, "", errors.New("network error"))
		home := t.TempDir()
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = "dev" })

		// Act
		latest, ok := fetchUpdateIfDue(home)

		// Assert
		assert.False(t, ok)
		assert.Empty(t, latest)
	})

	t.Run("returns false when already up to date", func(t *testing.T) {
		// Arrange
		stubFetchLatestVersion(t, "v1.0.0", nil)
		home := t.TempDir()
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = "dev" })

		// Act
		latest, ok := fetchUpdateIfDue(home)

		// Assert
		assert.False(t, ok)
		assert.Empty(t, latest)
	})

	t.Run("saves LastUpdateCheck after fetch", func(t *testing.T) {
		// Arrange
		stubFetchLatestVersion(t, "v1.0.0", nil)
		home := t.TempDir()
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = "dev" })
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
		stubFetchLatestVersion(t, "v1.1.0", nil)
		home := t.TempDir()
		buildinfo.Version = "v1.0.0"
		t.Cleanup(func() { buildinfo.Version = "dev" })

		// Act
		latest, ok := fetchUpdateIfDue(home)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, "v1.1.0", latest)
	})
}

func Test_notifyUpdate(t *testing.T) {
	buildinfo.Version = "v1.0.0"
	t.Cleanup(func() { buildinfo.Version = "dev" })

	t.Run("prints one-liner to stderr when not a terminal", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, false)

		var errBuf bytes.Buffer
		orig := upgradeStderr
		upgradeStderr = &errBuf
		t.Cleanup(func() { upgradeStderr = orig })

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

		var updateCalledWith string
		orig := performUpdate
		performUpdate = func(v string) error {
			updateCalledWith = v
			return nil
		}
		t.Cleanup(func() { performUpdate = orig })

		origStdin := upgradeStdin
		upgradeStdin = strings.NewReader("y\n")
		t.Cleanup(func() { upgradeStdin = origStdin })

		var errBuf bytes.Buffer
		origStderr := upgradeStderr
		upgradeStderr = &errBuf
		t.Cleanup(func() { upgradeStderr = origStderr })

		var exitCode int
		origExit := osExit
		osExit = func(code int) { exitCode = code }
		t.Cleanup(func() { osExit = origExit })

		// Act
		notifyUpdate("v1.1.0")

		// Assert
		assert.Equal(t, "v1.1.0", updateCalledWith)
		assert.Contains(t, errBuf.String(), "updated to v1.1.0")
		assert.Equal(t, 0, exitCode)
	})

	t.Run("exits with code 1 when terminal, user confirms, and update fails", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)

		stubPerformUpdate(t, errors.New("network error"))

		origStdin := upgradeStdin
		upgradeStdin = strings.NewReader("y\n")
		t.Cleanup(func() { upgradeStdin = origStdin })

		var errBuf bytes.Buffer
		origStderr := upgradeStderr
		upgradeStderr = &errBuf
		t.Cleanup(func() { upgradeStderr = origStderr })

		var exitCode int
		origExit := osExit
		osExit = func(code int) { exitCode = code }
		t.Cleanup(func() { osExit = origExit })

		// Act
		notifyUpdate("v1.1.0")

		// Assert
		assert.Contains(t, errBuf.String(), "update failed")
		assert.Equal(t, 1, exitCode)
	})

	t.Run("skips update when terminal and user declines", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)

		var updateCalled bool
		orig := performUpdate
		performUpdate = func(v string) error {
			updateCalled = true
			return nil
		}
		t.Cleanup(func() { performUpdate = orig })

		origStdin := upgradeStdin
		upgradeStdin = strings.NewReader("n\n")
		t.Cleanup(func() { upgradeStdin = origStdin })

		upgradeStderr = io.Discard

		// Act
		notifyUpdate("v1.1.0")

		// Assert
		assert.False(t, updateCalled)
	})
}
