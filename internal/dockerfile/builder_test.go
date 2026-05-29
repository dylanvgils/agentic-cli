package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStageBuilder_build(t *testing.T) {
	// Act
	stage := NewStage(From{Image: "base", As: "tool"}).
		Add(Env{Key: "FOO", Value: "bar"}).
		Add(User{Name: "app"}).
		Build()

	// Assert
	assert.Equal(t, From{Image: "base", As: "tool"}, stage.From)
	assert.Len(t, stage.Instructions, 2)
}

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

	t.Run("with comment", func(t *testing.T) {
		// Act
		stage := NewStage(From{Image: "base", As: "tool"}).
			Add(C("host user ID for container user mapping", Arg{Key: "HOST_UID", Default: "1000"})).
			Build()

		// Assert
		located, ok := stage.Instructions[0].(Located)
		assert.True(t, ok)
		assert.Equal(t, "host user ID for container user mapping", located.Comment)
		assert.Contains(t, located.Source, "builder_test.go:")
		assert.Equal(t, Arg{Key: "HOST_UID", Default: "1000"}, located.Inst)
	})

	t.Run("without comment backward compat", func(t *testing.T) {
		// Act
		stage := NewStage(From{Image: "base", As: "tool"}).
			Add(Env{Key: "FOO", Value: "bar"}).
			Build()

		// Assert
		located, ok := stage.Instructions[0].(Located)
		assert.True(t, ok)
		assert.Equal(t, "", located.Comment)
	})

	t.Run("C located no double wrap", func(t *testing.T) {
		// Act
		stage := NewStage(From{Image: "base", As: "tool"}).
			Add(C("a comment", User{Name: "claude"})).
			Build()

		// Assert — exactly one Located, Inst is User (not a nested Located)
		located, ok := stage.Instructions[0].(Located)
		assert.True(t, ok)
		assert.Equal(t, "a comment", located.Comment)
		assert.IsType(t, User{}, located.Inst)
	})

	t.Run("comment rendered in Dockerfile", func(t *testing.T) {
		// Arrange
		stage := NewStage(From{Image: "base", As: "tool"}).
			Add(C("host user ID", Arg{Key: "HOST_UID", Default: "1000"})).
			Build()

		// Act
		result := File{Stages: []Stage{stage}}.Render()

		// Assert
		assert.Contains(t, result, "# host user ID\n")
		assert.Contains(t, result, "ARG HOST_UID=1000")
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

	t.Run("already located fills source without double wrap", func(t *testing.T) {
		// Arrange
		inst := C("a comment", Env{Key: "FOO", Value: "bar"})

		// Act
		result := withSource("tools/bases.go:42", inst)

		// Assert
		located, ok := result.(Located)
		assert.True(t, ok)
		assert.Equal(t, "tools/bases.go:42", located.Source)
		assert.Equal(t, "a comment", located.Comment)
		assert.IsType(t, Env{}, located.Inst)
	})

	t.Run("empty source returns unwrapped", func(t *testing.T) {
		// Arrange
		inst := Env{Key: "FOO", Value: "bar"}

		// Act + Assert
		assert.Equal(t, inst, withSource("", inst))
	})
}
