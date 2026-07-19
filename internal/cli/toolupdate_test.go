package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_toolUpdateShouldCheck(t *testing.T) {
	t.Run("never checked returns true", func(t *testing.T) {
		// Act
		result := toolUpdateShouldCheck(nil, "claude")

		// Assert
		assert.True(t, result)
	})

	t.Run("recently checked returns false", func(t *testing.T) {
		// Arrange
		last := map[string]time.Time{"claude": time.Now().Add(-1 * time.Hour)}

		// Act
		result := toolUpdateShouldCheck(last, "claude")

		// Assert
		assert.False(t, result)
	})

	t.Run("past interval returns true", func(t *testing.T) {
		// Arrange
		last := map[string]time.Time{"claude": time.Now().Add(-7 * time.Hour)}

		// Act
		result := toolUpdateShouldCheck(last, "claude")

		// Assert
		assert.True(t, result)
	})
}

func Test_fetchToolUpdateIfDue(t *testing.T) {
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
		installed, latest, ok := fetchToolUpdateIfDue(home, "claude", "agentic-claude")

		// Assert
		assert.False(t, fetchCalled)
		assert.False(t, ok)
		assert.Empty(t, installed)
		assert.Empty(t, latest)
	})

	t.Run("returns not ok when image not found", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		stubInspectImage(t, nil, nil)

		// Act
		_, _, ok := fetchToolUpdateIfDue(home, "claude", "agentic-claude")

		// Assert
		assert.False(t, ok)
	})

	t.Run("returns not ok when inspect errors", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		stubInspectImage(t, nil, errors.New("docker error"))

		// Act
		_, _, ok := fetchToolUpdateIfDue(home, "claude", "agentic-claude")

		// Assert
		assert.False(t, ok)
	})

	t.Run("returns not ok when fetch fails", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		stubInspectImage(t, &docker.ImageInfo{Version: "1.2.3"}, nil)
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) { return "", false, false })

		// Act
		_, _, ok := fetchToolUpdateIfDue(home, "claude", "agentic-claude")

		// Assert
		assert.False(t, ok)
	})

	t.Run("returns not ok when already up to date", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		stubInspectImage(t, &docker.ImageInfo{Version: "1.2.3"}, nil)
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) { return "1.2.3", false, true })

		// Act
		_, _, ok := fetchToolUpdateIfDue(home, "claude", "agentic-claude")

		// Assert
		assert.False(t, ok)
	})

	t.Run("saves LastToolVersionCheck only after successful fetch", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		stubInspectImage(t, &docker.ImageInfo{Version: "1.2.3"}, nil)
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) { return "1.3.0", true, true })
		before := time.Now()

		// Act
		fetchToolUpdateIfDue(home, "claude", "agentic-claude")

		// Assert
		cfg, err := config.LoadConfig(home)
		require.NoError(t, err)
		require.Contains(t, cfg.LastToolVersionCheck, "claude")
		assert.True(t, cfg.LastToolVersionCheck["claude"].After(before))
	})

	t.Run("does not save timestamp when fetch fails", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		stubInspectImage(t, &docker.ImageInfo{Version: "1.2.3"}, nil)
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) { return "", false, false })

		// Act
		fetchToolUpdateIfDue(home, "claude", "agentic-claude")

		// Assert
		cfg, err := config.LoadConfig(home)
		require.NoError(t, err)
		assert.NotContains(t, cfg.LastToolVersionCheck, "claude")
	})

	t.Run("returns installed and latest when update available", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		stubInspectImage(t, &docker.ImageInfo{Version: "1.2.3"}, nil)
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) { return "1.3.0", true, true })

		// Act
		installed, latest, ok := fetchToolUpdateIfDue(home, "claude", "agentic-claude")

		// Assert
		assert.True(t, ok)
		assert.Equal(t, "1.2.3", installed)
		assert.Equal(t, "1.3.0", latest)
	})
}

func Test_notifyToolUpdate(t *testing.T) {
	t.Run("prints one-liner and returns false when not a terminal", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, false)

		var errBuf bytes.Buffer
		orig := toolUpdateStderr
		toolUpdateStderr = &errBuf
		t.Cleanup(func() { toolUpdateStderr = orig })

		// Act
		confirmed := notifyToolUpdate("claude", "1.2.3", "1.3.0")

		// Assert
		assert.False(t, confirmed)
		out := errBuf.String()
		assert.Contains(t, out, "claude")
		assert.Contains(t, out, "1.3.0")
		assert.Contains(t, out, "1.2.3")
		assert.Contains(t, out, "agentic update claude")
	})

	t.Run("returns true when terminal and user confirms", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)
		stubToolUpdateStdin(t, "y\n")

		toolUpdateStderr = io.Discard
		t.Cleanup(func() { toolUpdateStderr = os.Stderr })

		// Act
		confirmed := notifyToolUpdate("claude", "1.2.3", "1.3.0")

		// Assert
		assert.True(t, confirmed)
	})

	t.Run("returns false when terminal and user declines", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)
		stubToolUpdateStdin(t, "n\n")

		toolUpdateStderr = io.Discard
		t.Cleanup(func() { toolUpdateStderr = os.Stderr })

		// Act
		confirmed := notifyToolUpdate("claude", "1.2.3", "1.3.0")

		// Assert
		assert.False(t, confirmed)
	})
}

func Test_applyToolUpdate(t *testing.T) {
	t.Run("updates using recovered build options", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{Version: "1.2.3"}, nil)

		var updateCalledWith string
		stubUpdateTool(t, func(tool, image string, _ tools.BuildOptions) error {
			updateCalledWith = tool + ":" + image
			return nil
		})

		// Act
		err := applyToolUpdate("claude", "agentic-claude")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "claude:agentic-claude", updateCalledWith)
	})

	t.Run("returns formatted error when update fails", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{Version: "1.2.3"}, nil)
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error { return errors.New("build failed") })

		// Act
		err := applyToolUpdate("claude", "agentic-claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "update failed")
		assert.Contains(t, err.Error(), "agentic update claude")
	})
}

func Test_checkToolUpdate(t *testing.T) {
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
		err := checkToolUpdate(home, rc, "claude", "agentic-claude")

		// Assert
		require.NoError(t, err)
		assert.False(t, fetchCalled)
	})

	t.Run("runs check when check_updates is nil", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		rc := &config.AgenticRC{}
		stubInspectImage(t, &docker.ImageInfo{Version: "1.2.3"}, nil)
		var fetchCalled bool
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) {
			fetchCalled = true
			return "1.2.3", false, true
		})

		// Act
		err := checkToolUpdate(home, rc, "claude", "agentic-claude")

		// Assert
		require.NoError(t, err)
		assert.True(t, fetchCalled)
	})

	t.Run("runs check when check_updates is true", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		enabled := true
		rc := &config.AgenticRC{Run: config.RCRun{CheckUpdates: &enabled}}
		stubInspectImage(t, &docker.ImageInfo{Version: "1.2.3"}, nil)
		var fetchCalled bool
		stubLatestToolVersion(t, func(_, _ string) (string, bool, bool) {
			fetchCalled = true
			return "1.2.3", false, true
		})

		// Act
		err := checkToolUpdate(home, rc, "claude", "agentic-claude")

		// Assert
		require.NoError(t, err)
		assert.True(t, fetchCalled)
	})
}
