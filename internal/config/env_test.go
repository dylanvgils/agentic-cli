package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvVersionVar(t *testing.T) {
	t.Run("java", func(t *testing.T) {
		// Act
		result := EnvVersionVar("java")

		// Assert
		assert.Equal(t, envPrefix+"JAVA_VERSION", result)
	})

	t.Run("dotnet", func(t *testing.T) {
		// Act
		result := EnvVersionVar("dotnet")

		// Assert
		assert.Equal(t, envPrefix+"DOTNET_VERSION", result)
	})

	t.Run("go", func(t *testing.T) {
		// Act
		result := EnvVersionVar("go")

		// Assert
		assert.Equal(t, envPrefix+"GO_VERSION", result)
	})
}
