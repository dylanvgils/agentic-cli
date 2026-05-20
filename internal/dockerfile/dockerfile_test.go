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
	assert.Equal(t, "FROM debian:bookworm-slim AS base\nENV FOO=bar\n", result)
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
		"ARG NODE_VERSION=24\nFROM node:${NODE_VERSION}-bookworm-slim AS base\n\nFROM base AS tool\nUSER app\n",
		result,
	)
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
