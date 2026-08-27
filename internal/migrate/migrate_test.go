package migrate

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	t.Run("creates and baselines a fresh TOOL_HOME to the latest version", func(t *testing.T) {
		// Arrange
		toolHome := filepath.Join(t.TempDir(), "does-not-exist-yet")
		pending := []Migration{
			{Version: 1, Description: "one", Apply: func(string) error { return nil }},
			{Version: 2, Description: "two", Apply: func(string) error { return nil }},
		}

		// Act
		applied, err := run(toolHome, pending)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, applied)
		s, err := loadState(toolHome)
		require.NoError(t, err)
		assert.Equal(t, 2, s.Version)
	})

	t.Run("errors when TOOL_HOME predates the oldest pending migration", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		var called bool
		pending := []Migration{
			{Version: 3, Description: "three", Apply: func(string) error { called = true; return nil }},
		}
		require.NoError(t, saveState(toolHome, state{Version: 1}))

		// Act
		applied, err := run(toolHome, pending)

		// Assert
		require.Error(t, err)
		assert.ErrorContains(t, err, "schema version 1")
		assert.ErrorContains(t, err, "v3")
		assert.False(t, called)
		assert.Empty(t, applied)
	})

	t.Run("applies pending migrations when TOOL_HOME sits exactly at the oldest pending migration's floor", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		pending := []Migration{
			{Version: 3, Description: "three", Apply: func(string) error { return nil }},
		}
		require.NoError(t, saveState(toolHome, state{Version: 2}))

		// Act
		applied, err := run(toolHome, pending)

		// Assert
		require.NoError(t, err)
		assert.Len(t, applied, 1)
	})

	t.Run("a retried run re-applies only the previously failed migration, not earlier successes", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		var firstCalls, secondCalls int
		failSecond := true
		pending := []Migration{
			{Version: 1, Description: "one", Apply: func(string) error { firstCalls++; return nil }},
			{Version: 2, Description: "two", Apply: func(string) error {
				secondCalls++
				if failSecond {
					return errors.New("boom")
				}
				return nil
			}},
		}

		// Act
		_, firstErr := run(toolHome, pending)
		failSecond = false
		applied, secondErr := run(toolHome, pending)

		// Assert
		require.Error(t, firstErr)
		require.NoError(t, secondErr)
		assert.Equal(t, 1, firstCalls)
		assert.Equal(t, 2, secondCalls)
		assert.Len(t, applied, 1)
		assert.Equal(t, 2, applied[0].Version)
	})
}

func TestApplyPending(t *testing.T) {
	t.Run("applies every pending migration newer than current.Version", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		var calls []int
		pending := []Migration{
			{Version: 1, Description: "one", Apply: func(string) error { calls = append(calls, 1); return nil }},
			{Version: 2, Description: "two", Apply: func(string) error { calls = append(calls, 2); return nil }},
		}

		// Act
		applied, err := applyPending(toolHome, pending, state{Version: 0})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []int{1, 2}, calls)
		assert.Len(t, applied, 2)
		s, err := loadState(toolHome)
		require.NoError(t, err)
		assert.Equal(t, 2, s.Version)
	})

	t.Run("skips migrations at or below current.Version", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		var calls int
		pending := []Migration{
			{Version: 1, Description: "one", Apply: func(string) error { calls++; return nil }},
		}

		// Act
		applied, err := applyPending(toolHome, pending, state{Version: 1})

		// Assert
		require.NoError(t, err)
		assert.Empty(t, applied)
		assert.Zero(t, calls)
	})

	t.Run("stops at the first failing migration and does not advance past it", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		var thirdCalled bool
		pending := []Migration{
			{Version: 1, Description: "ok", Apply: func(string) error { return nil }},
			{Version: 2, Description: "failing", Apply: func(string) error { return errors.New("boom") }},
			{Version: 3, Description: "ok2", Apply: func(string) error { thirdCalled = true; return nil }},
		}

		// Act
		applied, err := applyPending(toolHome, pending, state{Version: 0})

		// Assert
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
		assert.False(t, thirdCalled)
		assert.Len(t, applied, 1)
		s, loadErr := loadState(toolHome)
		require.NoError(t, loadErr)
		assert.Equal(t, 1, s.Version)
	})
}

func TestState_roundTrip(t *testing.T) {
	t.Run("loadState on a missing file returns the zero value", func(t *testing.T) {
		// Act
		s, err := loadState(t.TempDir())

		// Assert
		require.NoError(t, err)
		assert.Equal(t, state{}, s)
	})

	t.Run("saveState persists a value loadState can read back", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()

		// Act
		err := saveState(toolHome, state{Version: 3})

		// Assert
		require.NoError(t, err)
		s, err := loadState(toolHome)
		require.NoError(t, err)
		assert.Equal(t, 3, s.Version)
	})
}
