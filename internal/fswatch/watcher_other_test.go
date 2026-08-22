//go:build !linux && !darwin

package fswatch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWatcher(t *testing.T) {
	t.Run("Start fails with a clear error", func(t *testing.T) {
		// Arrange
		w := New([]string{"/tmp"}, NewLogger(&syncBuffer{}), Options{})

		// Act
		err := w.Start()

		// Assert
		assert.ErrorContains(t, err, "not supported on this host OS")
	})

	t.Run("Stop is a no-op", func(t *testing.T) {
		// Arrange
		w := New([]string{"/tmp"}, NewLogger(&syncBuffer{}), Options{})

		// Act + Assert
		assert.NotPanics(t, w.Stop)
	})
}
