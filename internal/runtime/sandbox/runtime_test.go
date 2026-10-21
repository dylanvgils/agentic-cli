package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeNames(t *testing.T) {
	// Act
	names := RuntimeNames()

	// Assert
	assert.Equal(t, "docker or microsandbox", names)
}

func TestParseRuntime(t *testing.T) {
	t.Run("known name returns its runtime", func(t *testing.T) {
		// Act
		runtime, err := ParseRuntime("microsandbox")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, RuntimeMicrosandbox, runtime)
	})

	t.Run("unknown name lists the supported runtimes", func(t *testing.T) {
		// Act
		_, err := ParseRuntime("podman")

		// Assert
		assert.EqualError(t, err, `unknown runtime "podman": expected docker or microsandbox`)
	})
}
