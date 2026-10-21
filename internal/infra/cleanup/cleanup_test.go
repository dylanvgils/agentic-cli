package cleanup

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCapture(t *testing.T) {
	t.Run("sets err when fn fails and err is nil", func(t *testing.T) {
		// Arrange
		fnErr := errors.New("fn failed")
		err := error(nil)

		// Act
		Capture(&err, func() error { return fnErr })

		// Assert
		assert.ErrorIs(t, err, fnErr)
	})

	t.Run("does not overwrite existing err", func(t *testing.T) {
		// Arrange
		existing := errors.New("existing")
		err := existing

		// Act
		Capture(&err, func() error { return errors.New("fn failed") })

		// Assert
		assert.ErrorIs(t, err, existing)
	})

	t.Run("leaves err nil when fn succeeds", func(t *testing.T) {
		// Arrange
		err := error(nil)

		// Act
		Capture(&err, func() error { return nil })

		// Assert
		assert.NoError(t, err)
	})
}
