package toolupdate

import (
	"errors"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_shouldCheck(t *testing.T) {
	t.Run("never checked returns true", func(t *testing.T) {
		// Act
		result := shouldCheck(nil, "claude")

		// Assert
		assert.True(t, result)
	})

	t.Run("recently checked returns false", func(t *testing.T) {
		// Arrange
		last := map[string]time.Time{"claude": time.Now().Add(-1 * time.Hour)}

		// Act
		result := shouldCheck(last, "claude")

		// Assert
		assert.False(t, result)
	})

	t.Run("past interval returns true", func(t *testing.T) {
		// Arrange
		last := map[string]time.Time{"claude": time.Now().Add(-7 * time.Hour)}

		// Act
		result := shouldCheck(last, "claude")

		// Assert
		assert.True(t, result)
	})
}

func Test_fetchIfDue(t *testing.T) {
	t.Run("returns not ok when within interval", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		lastCheck := time.Now().Add(-1 * time.Hour)
		cfg := &config.CliConfig{LastToolVersionCheck: map[string]time.Time{"claude": lastCheck}}
		require.NoError(t, cfg.Save(home))
		var fetchCalled bool
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) {
			fetchCalled = true
			return "", false, false
		})

		// Act
		installed, latest, ok := New(&fakeDocker{}).fetchIfDue(home, "claude", "agentic-claude")

		// Assert
		assert.False(t, fetchCalled)
		assert.False(t, ok)
		assert.Empty(t, installed)
		assert.Empty(t, latest)
	})

	t.Run("returns not ok when image not found", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		d := &fakeDocker{inspectImage: inspectReturns(nil, nil)}

		// Act
		_, _, ok := New(d).fetchIfDue(home, "claude", "agentic-claude")

		// Assert
		assert.False(t, ok)
	})

	t.Run("returns not ok when inspect errors", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		d := &fakeDocker{inspectImage: inspectReturns(nil, errors.New("docker error"))}

		// Act
		_, _, ok := New(d).fetchIfDue(home, "claude", "agentic-claude")

		// Assert
		assert.False(t, ok)
	})

	t.Run("returns not ok when fetch fails", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.2.3"}, nil)}
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) { return "", false, false })

		// Act
		_, _, ok := New(d).fetchIfDue(home, "claude", "agentic-claude")

		// Assert
		assert.False(t, ok)
	})

	t.Run("returns not ok when already up to date", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.2.3"}, nil)}
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) { return "1.2.3", false, true })

		// Act
		_, _, ok := New(d).fetchIfDue(home, "claude", "agentic-claude")

		// Assert
		assert.False(t, ok)
	})

	t.Run("saves LastToolVersionCheck only after successful fetch", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.2.3"}, nil)}
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) { return "1.3.0", true, true })
		before := time.Now()

		// Act
		_, _, ok := New(d).fetchIfDue(home, "claude", "agentic-claude")

		// Assert
		require.True(t, ok)
		cfg, err := config.LoadConfig(home)
		require.NoError(t, err)
		require.Contains(t, cfg.LastToolVersionCheck, "claude")
		assert.True(t, cfg.LastToolVersionCheck["claude"].After(before))
	})

	t.Run("does not save timestamp when fetch fails", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.2.3"}, nil)}
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) { return "", false, false })

		// Act
		_, _, ok := New(d).fetchIfDue(home, "claude", "agentic-claude")

		// Assert
		require.False(t, ok)
		cfg, err := config.LoadConfig(home)
		require.NoError(t, err)
		assert.NotContains(t, cfg.LastToolVersionCheck, "claude")
	})

	t.Run("returns installed and latest when update available", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.2.3"}, nil)}
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) { return "1.3.0", true, true })

		// Act
		installed, latest, ok := New(d).fetchIfDue(home, "claude", "agentic-claude")

		// Assert
		assert.True(t, ok)
		assert.Equal(t, "1.2.3", installed)
		assert.Equal(t, "1.3.0", latest)
	})
}

func TestCheck(t *testing.T) {
	noopUpdate := func(string, string) error { return nil }
	confirmYes := func(string, string, string) bool { return true }
	confirmNo := func(string, string, string) bool { return false }

	t.Run("skips when check_updates is false in rc", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		disabled := false
		rc := &config.AgenticRC{Run: config.RCRun{CheckUpdates: &disabled}}
		var fetchCalled bool
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) {
			fetchCalled = true
			return "", false, false
		})

		// Act
		err := New(&fakeDocker{}).Check(home, rc, "claude", "agentic-claude", confirmNo, noopUpdate)

		// Assert
		require.NoError(t, err)
		assert.False(t, fetchCalled)
	})

	t.Run("runs check when check_updates is nil", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		rc := &config.AgenticRC{}
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.2.3"}, nil)}
		var fetchCalled bool
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) {
			fetchCalled = true
			return "1.2.3", false, true
		})

		// Act
		err := New(d).Check(home, rc, "claude", "agentic-claude", confirmNo, noopUpdate)

		// Assert
		require.NoError(t, err)
		assert.True(t, fetchCalled)
	})

	t.Run("runs check when check_updates is true", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		enabled := true
		rc := &config.AgenticRC{Run: config.RCRun{CheckUpdates: &enabled}}
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.2.3"}, nil)}
		var fetchCalled bool
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) {
			fetchCalled = true
			return "1.2.3", false, true
		})

		// Act
		err := New(d).Check(home, rc, "claude", "agentic-claude", confirmNo, noopUpdate)

		// Assert
		require.NoError(t, err)
		assert.True(t, fetchCalled)
	})

	t.Run("calls update with tool and image when confirmed", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		rc := &config.AgenticRC{}
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.2.3"}, nil)}
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) { return "1.3.0", true, true })
		var updateCalledWith string
		update := func(tool, image string) error {
			updateCalledWith = tool + ":" + image
			return nil
		}

		var offered string
		confirm := func(tool, installed, latest string) bool {
			offered = tool + " " + installed + " -> " + latest
			return true
		}

		// Act
		err := New(d).Check(home, rc, "claude", "agentic-claude", confirm, update)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "claude 1.2.3 -> 1.3.0", offered)
		assert.Equal(t, "claude:agentic-claude", updateCalledWith)
	})

	t.Run("returns formatted error when update fails", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		rc := &config.AgenticRC{}
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.2.3"}, nil)}
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) { return "1.3.0", true, true })
		update := func(string, string) error { return errors.New("build failed") }

		// Act
		err := New(d).Check(home, rc, "claude", "agentic-claude", confirmYes, update)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "update failed")
		assert.Contains(t, err.Error(), "build failed")
		assert.Contains(t, err.Error(), "agentic update claude")
	})

	t.Run("does not call update when not confirmed", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		rc := &config.AgenticRC{}
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.2.3"}, nil)}
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) { return "1.3.0", true, true })
		called := false
		update := func(string, string) error {
			called = true
			return nil
		}

		// Act
		err := New(d).Check(home, rc, "claude", "agentic-claude", confirmNo, update)

		// Assert
		require.NoError(t, err)
		assert.False(t, called)
	})
}

func TestMarkChecked(t *testing.T) {
	t.Run("saves timestamp when none recorded", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		before := time.Now()

		// Act
		err := MarkChecked(home, "claude")

		// Assert
		require.NoError(t, err)
		cfg, err := config.LoadConfig(home)
		require.NoError(t, err)
		require.Contains(t, cfg.LastToolVersionCheck, "claude")
		assert.False(t, cfg.LastToolVersionCheck["claude"].Before(before))
	})

	t.Run("keeps other tools' timestamps", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		other := time.Now().Add(-2 * time.Hour)
		cfg := &config.CliConfig{LastToolVersionCheck: map[string]time.Time{"copilot": other}}
		require.NoError(t, cfg.Save(home))

		// Act
		err := MarkChecked(home, "claude")

		// Assert
		require.NoError(t, err)
		reloaded, err := config.LoadConfig(home)
		require.NoError(t, err)
		assert.Contains(t, reloaded.LastToolVersionCheck, "claude")
		assert.True(t, other.Equal(reloaded.LastToolVersionCheck["copilot"]))
	})
}
