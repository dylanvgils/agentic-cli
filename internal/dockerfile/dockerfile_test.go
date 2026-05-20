package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFile_singleStage(t *testing.T) {
	// Arrange
	f := File{
		Stages: []Stage{
			{
				From:         From{Image: "debian:bookworm-slim", As: "base"},
				Instructions: []Instruction{Env{Key: "FOO", Value: "bar"}},
			},
		},
	}

	// Act
	result := f.Render()

	// Assert
	assert.Equal(t, "FROM debian:bookworm-slim AS base\n\nENV FOO=bar\n", result)
}

func TestFile_multiStage(t *testing.T) {
	// Arrange
	f := File{
		Stages: []Stage{
			{
				GlobalArgs: []Arg{{Key: "NODE_VERSION", Default: "24"}},
				From:       From{Image: "node:${NODE_VERSION}-bookworm-slim", As: "base"},
			},
			{
				From:         From{Image: "base", As: "tool"},
				Instructions: []Instruction{User{Name: "app"}},
			},
		},
	}

	// Act
	result := f.Render()

	// Assert
	assert.Equal(t,
		"ARG NODE_VERSION=24\nFROM node:${NODE_VERSION}-bookworm-slim AS base\n\nFROM base AS tool\n\nUSER app\n",
		result,
	)
}

// --- StageBuilder ---

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
	assert.Contains(t, located.Source, "dockerfile_test.go:")
	assert.Equal(t, Env{Key: "FOO", Value: "bar"}, located.Inst)
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

func TestFile_globalArgsBeforeFrom(t *testing.T) {
	// Arrange
	f := File{
		Stages: []Stage{
			{
				GlobalArgs: []Arg{
					{Key: "NODE_VERSION", Default: "24"},
					{Key: "TARGETARCH"},
				},
				From: From{Image: "node:${NODE_VERSION}-slim", As: "base"},
			},
		},
	}

	// Act
	result := f.Render()

	// Assert
	assert.Equal(t, "ARG NODE_VERSION=24\nARG TARGETARCH\nFROM node:${NODE_VERSION}-slim AS base\n", result)
}
