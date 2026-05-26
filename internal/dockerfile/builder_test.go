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

func TestStageBuilder_addCapturesLocation(t *testing.T) {
	// Act
	stage := NewStage(From{Image: "base", As: "tool"}).
		Add(Env{Key: "FOO", Value: "bar"}).
		Build()

	// Assert
	located, ok := stage.Instructions[0].(Located)
	assert.True(t, ok)
	assert.Contains(t, located.Source, "builder_test.go:")
	assert.Equal(t, Env{Key: "FOO", Value: "bar"}, located.Inst)
}

func TestStageBuilder_addWithComment(t *testing.T) {
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
}

func TestStageBuilder_addWithoutComment_backwardCompat(t *testing.T) {
	// Act
	stage := NewStage(From{Image: "base", As: "tool"}).
		Add(Env{Key: "FOO", Value: "bar"}).
		Build()

	// Assert
	located, ok := stage.Instructions[0].(Located)
	assert.True(t, ok)
	assert.Equal(t, "", located.Comment)
}

func TestStageBuilder_addCLocated_noDoubleWrap(t *testing.T) {
	// Act
	stage := NewStage(From{Image: "base", As: "tool"}).
		Add(C("a comment", User{Name: "claude"})).
		Build()

	// Assert — exactly one Located, Inst is User (not a nested Located)
	located, ok := stage.Instructions[0].(Located)
	assert.True(t, ok)
	assert.Equal(t, "a comment", located.Comment)
	assert.IsType(t, User{}, located.Inst)
}

func TestStageBuilder_addComment_renderedInDockerfile(t *testing.T) {
	// Arrange
	stage := NewStage(From{Image: "base", As: "tool"}).
		Add(C("host user ID", Arg{Key: "HOST_UID", Default: "1000"})).
		Build()

	// Act
	result := File{Stages: []Stage{stage}}.Render()

	// Assert
	assert.Contains(t, result, "# host user ID\n")
	assert.Contains(t, result, "ARG HOST_UID=1000")
}

func TestStageBuilder_addMultiple_appendsAllInOrder(t *testing.T) {
	// Act
	stage := NewStage(From{Image: "base", As: "tool"}).
		Add(Arg{Key: "HOST_UID", Default: "1000"}, Arg{Key: "HOST_GID", Default: "1000"}).
		Build()

	// Assert
	assert.Len(t, stage.Instructions, 2)
	assert.Equal(t, Arg{Key: "HOST_UID", Default: "1000"}, stage.Instructions[0].(Located).Inst)
	assert.Equal(t, Arg{Key: "HOST_GID", Default: "1000"}, stage.Instructions[1].(Located).Inst)
}

func TestWithSource_plainInstruction_wrapsWithSource(t *testing.T) {
	// Arrange
	inst := Env{Key: "FOO", Value: "bar"}

	// Act
	result := withSource("tools/bases.go:42", inst)

	// Assert
	located, ok := result.(Located)
	assert.True(t, ok)
	assert.Equal(t, "tools/bases.go:42", located.Source)
	assert.Equal(t, inst, located.Inst)
}

func TestWithSource_alreadyLocated_fillsSourceWithoutDoubleWrap(t *testing.T) {
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
}

func TestWithSource_emptySource_returnsUnwrapped(t *testing.T) {
	// Arrange
	inst := Env{Key: "FOO", Value: "bar"}

	// Act + Assert
	assert.Equal(t, inst, withSource("", inst))
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
