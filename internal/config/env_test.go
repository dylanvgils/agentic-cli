package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvVersionVar(t *testing.T) {
	t.Run("java", func(t *testing.T) {
		assert.Equal(t, envPrefix+"JAVA_VERSION", EnvVersionVar("java"))
	})

	t.Run("dotnet", func(t *testing.T) {
		assert.Equal(t, envPrefix+"DOTNET_VERSION", EnvVersionVar("dotnet"))
	})

	t.Run("go", func(t *testing.T) {
		assert.Equal(t, envPrefix+"GO_VERSION", EnvVersionVar("go"))
	})
}
