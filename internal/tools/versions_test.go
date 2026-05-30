package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_versionsForExtra(t *testing.T) {
	v := Versions{Java: "25", Dotnet: "10", Go: "1.26.3"}

	t.Run("java returns java field", func(t *testing.T) {
		// Act
		result := v.ForExtra("java")

		// Assert
		assert.Equal(t, "25", result)
	})

	t.Run("dotnet returns dotnet field", func(t *testing.T) {
		// Act
		result := v.ForExtra("dotnet")

		// Assert
		assert.Equal(t, "10", result)
	})

	t.Run("go returns go field", func(t *testing.T) {
		// Act
		result := v.ForExtra("go")

		// Assert
		assert.Equal(t, "1.26.3", result)
	})

	t.Run("unknown name returns empty string", func(t *testing.T) {
		// Act
		result := v.ForExtra("ruby")

		// Assert
		assert.Equal(t, "", result)
	})
}
