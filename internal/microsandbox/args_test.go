package microsandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_arg(t *testing.T) {
	t.Run("builds flag with value", func(t *testing.T) {
		// Act
		result := arg("mount-dir", "/host:/guest:ro")

		// Assert
		assert.Equal(t, "--mount-dir=/host:/guest:ro", result)
	})

	t.Run("keeps an = in the value", func(t *testing.T) {
		// Act
		result := arg("env", "FOO=bar")

		// Assert
		assert.Equal(t, "--env=FOO=bar", result)
	})

	t.Run("builds flag without value", func(t *testing.T) {
		// Act
		result := arg("quiet")

		// Assert
		assert.Equal(t, "--quiet", result)
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
		assert.Panics(t, func() { arg("env", "a", "b") })
	})
}
