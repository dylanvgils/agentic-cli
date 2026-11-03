package run

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_requireVersion(t *testing.T) {
	t.Run("accepts the minimum", func(t *testing.T) {
		// Act
		err := requireVersion("Docker Engine", "26.0", "26.0")

		// Assert
		assert.NoError(t, err)
	})

	t.Run("refuses an older version", func(t *testing.T) {
		// Act
		err := requireVersion("Docker Engine", "25.0.5", "26.0")

		// Assert
		assert.EqualError(t, err, "Docker Engine 25.0.5 is older than the minimum 26.0; update it and run again")
	})

	t.Run("refuses an unreadable version", func(t *testing.T) {
		// Act
		err := requireVersion("Docker Engine", "dev", "26.0")

		// Assert
		assert.EqualError(t, err, `read the Docker Engine version: not a version: "dev"`)
	})
}
