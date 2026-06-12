package cmd

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunUpgrade(t *testing.T) {
	t.Run("prints already up to date when no newer version", func(t *testing.T) {
		// Arrange
		stubFetchLatestVersion(t, "v1.0.0", nil)
		stubPerformUpdate(t, nil)
		version = "v1.0.0"
		t.Cleanup(func() { version = "dev" })

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
		version = "v1.0.0"
		t.Cleanup(func() { version = "dev" })

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
		version = "v1.0.0"
		t.Cleanup(func() { version = "dev" })

		// Act
		err := runUpgrade(upgradeCmd, nil)

		// Assert
		assert.Error(t, err)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		// Arrange
		stubFetchLatestVersion(t, "v1.1.0", nil)
		stubPerformUpdate(t, errors.New("permission denied"))
		version = "v1.0.0"
		t.Cleanup(func() { version = "dev" })

		// Act
		err := runUpgrade(upgradeCmd, nil)

		// Assert
		assert.Error(t, err)
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

	t.Run("skips when within check interval", func(t *testing.T) {
		// Arrange
		var fetchCalled bool
		stubFetchLatestVersion(t, "v1.1.0", nil)
		orig := fetchLatestVersion
		fetchLatestVersion = func() (string, error) {
			fetchCalled = true
			return "v1.1.0", nil
		}
		t.Cleanup(func() { fetchLatestVersion = orig })

		home := t.TempDir()
		cfg := &config.CliConfig{LastUpdateCheck: time.Now().Add(-1 * time.Hour)}
		require.NoError(t, cfg.Save(home))

		version = "v1.0.0"
		t.Cleanup(func() { version = "dev" })

		// Act
		maybeNotifyUpdate(home)

		// Assert
		assert.False(t, fetchCalled)
	})

	t.Run("prints notice to stderr when not a terminal", func(t *testing.T) {
		// Arrange
		stubFetchLatestVersion(t, "v1.1.0", nil)
		stubIsTerminal(t, false)
		home := t.TempDir()

		version = "v1.0.0"
		t.Cleanup(func() { version = "dev" })

		var errBuf bytes.Buffer
		orig := upgradeStderr
		upgradeStderr = &errBuf
		t.Cleanup(func() { upgradeStderr = orig })

		// Act
		maybeNotifyUpdate(home)

		// Assert
		out := errBuf.String()
		assert.Contains(t, out, "v1.1.0")
		assert.Contains(t, out, "v1.0.0")
		assert.Contains(t, out, "upgrade")
	})

	t.Run("prompts and updates when terminal and user confirms", func(t *testing.T) {
		// Arrange
		stubFetchLatestVersion(t, "v1.1.0", nil)
		stubIsTerminal(t, true)
		home := t.TempDir()

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

		version = "v1.0.0"
		t.Cleanup(func() { version = "dev" })

		// Act
		maybeNotifyUpdate(home)

		// Assert
		assert.Equal(t, "v1.1.0", updateCalledWith)
		assert.Contains(t, errBuf.String(), "updated to v1.1.0")
	})

	t.Run("skips update when terminal and user declines", func(t *testing.T) {
		// Arrange
		stubFetchLatestVersion(t, "v1.1.0", nil)
		stubIsTerminal(t, true)
		home := t.TempDir()

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

		version = "v1.0.0"
		t.Cleanup(func() { version = "dev" })

		// Act
		maybeNotifyUpdate(home)

		// Assert
		assert.False(t, updateCalled)
	})

	t.Run("updates LastUpdateCheck in config after fetch", func(t *testing.T) {
		// Arrange
		stubFetchLatestVersion(t, "v1.0.0", nil)
		stubIsTerminal(t, false)
		upgradeStderr = io.Discard

		home := t.TempDir()
		version = "v1.0.0"
		t.Cleanup(func() { version = "dev" })

		before := time.Now()

		// Act
		maybeNotifyUpdate(home)

		// Assert
		cfg, err := config.LoadConfig(home)
		require.NoError(t, err)
		assert.True(t, cfg.LastUpdateCheck.After(before))
	})

	t.Run("silently skips when fetch fails", func(t *testing.T) {
		// Arrange
		stubFetchLatestVersion(t, "", errors.New("network error"))
		home := t.TempDir()
		version = "v1.0.0"
		t.Cleanup(func() { version = "dev" })

		// Act + Assert — must not panic or print anything
		assert.NotPanics(t, func() { maybeNotifyUpdate(home) })
	})
}

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
