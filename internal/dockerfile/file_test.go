package dockerfile

import (
	"strings"
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
	divider := strings.Repeat("#", dividerWidth)
	assert.Equal(t, divider+"\n# base\n"+divider+"\nFROM debian:bookworm-slim AS base\n\nENV FOO=bar\n", result)
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
	divider := strings.Repeat("#", dividerWidth)
	assert.Equal(t,
		divider+"\n# base\n"+divider+"\nARG NODE_VERSION=24\nFROM node:${NODE_VERSION}-bookworm-slim AS base\n\n"+
			divider+"\n# tool\n"+divider+"\nFROM base AS tool\n\nUSER app\n",
		result,
	)
}

func TestFile_dividerUsesStageAs(t *testing.T) {
	// Arrange
	f := File{
		Stages: []Stage{
			{From: From{Image: "debian:bookworm-slim", As: "base"}},
			{From: From{Image: "base", As: "myapp"}},
		},
	}

	// Act
	result := f.Render()

	// Assert
	divider := strings.Repeat("#", dividerWidth)
	assert.Contains(t, result, divider+"\n# myapp\n"+divider+"\n")
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
	divider := strings.Repeat("#", dividerWidth)
	assert.Equal(t, divider+"\n# base\n"+divider+"\nARG NODE_VERSION=24\nARG TARGETARCH\nFROM node:${NODE_VERSION}-slim AS base\n", result)
}
