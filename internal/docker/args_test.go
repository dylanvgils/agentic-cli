package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArg(t *testing.T) {
	t.Run("builds flag with value", func(t *testing.T) {
		// Act
		result := arg("volume", "/host:/container")

		// Assert
		assert.Equal(t, "--volume=/host:/container", result)
	})

	t.Run("builds flag without value", func(t *testing.T) {
		// Act
		result := arg("force")

		// Assert
		assert.Equal(t, "--force", result)
	})

	t.Run("empty name panics", func(t *testing.T) {
		// Act + Assert
		assert.Panics(t, func() { arg("") })
	})

	t.Run("dash prefix panics", func(t *testing.T) {
		// Act + Assert
		assert.Panics(t, func() { arg("-flag") })
	})

	t.Run("multiple values panics", func(t *testing.T) {
		// Act + Assert
		assert.Panics(t, func() { arg("filter", "a", "b") })
	})
}
