package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_versionsForLayer(t *testing.T) {
	v := Versions{Node: "22", Java: "25", Dotnet: "10", Go: "1.26.3"}

	t.Run("node returns node field", func(t *testing.T) {
		// Act
		result := v.ForLayer("node")

		// Assert
		assert.Equal(t, "22", result)
	})

	t.Run("java returns java field", func(t *testing.T) {
		// Act
		result := v.ForLayer("java")

		// Assert
		assert.Equal(t, "25", result)
	})

	t.Run("dotnet returns dotnet field", func(t *testing.T) {
		// Act
		result := v.ForLayer("dotnet")

		// Assert
		assert.Equal(t, "10", result)
	})

	t.Run("go returns go field", func(t *testing.T) {
		// Act
		result := v.ForLayer("go")

		// Assert
		assert.Equal(t, "1.26.3", result)
	})

	t.Run("unknown name returns empty string", func(t *testing.T) {
		// Act
		result := v.ForLayer("ruby")

		// Assert
		assert.Equal(t, "", result)
	})
}
