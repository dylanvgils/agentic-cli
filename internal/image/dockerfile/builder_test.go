package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStageBuilder_Add(t *testing.T) {
	t.Run("captures location", func(t *testing.T) {
		// Act
		stage := NewStage(From{Image: "base", As: "tool"}).
			Add(Env{Key: "FOO", Value: "bar"}).
			Build()

		// Assert
		located, ok := stage.Instructions[0].(Located)
		assert.True(t, ok)
		assert.Contains(t, located.Source, "builder_test.go:")
		assert.Equal(t, Env{Key: "FOO", Value: "bar"}, located.Inst)
	})

	t.Run("multiple appends all in order", func(t *testing.T) {
		// Act
		stage := NewStage(From{Image: "base", As: "tool"}).
			Add(Arg{Key: "HOST_UID", Default: "1000"}, Arg{Key: "HOST_GID", Default: "1000"}).
			Build()

		// Assert
		assert.Len(t, stage.Instructions, 2)
		assert.Equal(t, Arg{Key: "HOST_UID", Default: "1000"}, stage.Instructions[0].(Located).Inst)
		assert.Equal(t, Arg{Key: "HOST_GID", Default: "1000"}, stage.Instructions[1].(Located).Inst)
	})
}

func TestStageBuilder_addGlobalArg(t *testing.T) {
	// Act
	stage := NewStage(From{Image: "node:${NODE_VERSION}-slim", As: "base"}).
		AddGlobalArg(Arg{Key: "NODE_VERSION", Default: "24"}).
		Build()

	// Assert
	assert.Equal(t, []Arg{{Key: "NODE_VERSION", Default: "24"}}, stage.GlobalArgs)
	assert.Empty(t, stage.Instructions)
}

func TestWithSource(t *testing.T) {
	t.Run("plain instruction wraps with source", func(t *testing.T) {
		// Arrange
		inst := Env{Key: "FOO", Value: "bar"}

		// Act
		result := withSource("tools/bases.go:42", inst)

		// Assert
		located, ok := result.(Located)
		assert.True(t, ok)
		assert.Equal(t, "tools/bases.go:42", located.Source)
		assert.Equal(t, inst, located.Inst)
	})

	t.Run("empty source returns unwrapped", func(t *testing.T) {
		// Arrange
		inst := Env{Key: "FOO", Value: "bar"}

		// Act + Assert
		assert.Equal(t, inst, withSource("", inst))
	})
}
