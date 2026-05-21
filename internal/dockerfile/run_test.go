package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun_singleLine(t *testing.T) {
	// Act
	result := Run{Command: "apt-get update"}.Render()

	// Assert
	assert.Equal(t, "RUN apt-get update", result)
}

func TestRun_blocks_noComment(t *testing.T) {
	// Arrange
	run := Run{Blocks: []Block{
		{Lines: []string{"apt-get update"}},
		{Lines: []string{"apt-get install curl", "wget"}},
		{Lines: []string{"rm -rf /var/lib/apt/lists/*"}},
	}}

	// Act
	result := run.Render()

	// Assert
	assert.Equal(t, "RUN apt-get update \\\n  && apt-get install curl \\\n  wget \\\n  && rm -rf /var/lib/apt/lists/*", result)
}

func TestRun_blocks_withComment(t *testing.T) {
	// Arrange
	run := Run{Blocks: []Block{
		{Comment: "Update package list", Lines: []string{"apt-get update"}},
		{Comment: "Install packages", Lines: []string{"apt-get install curl"}},
	}}

	// Act
	result := run.Render()

	// Assert
	assert.Equal(t, "RUN \\\n  # Update package list\n  apt-get update \\\n  \\\n  # Install packages\n  && apt-get install curl", result)
}

func TestRun_multiLine(t *testing.T) {
	// Act
	result := Run{Lines: []string{"apt-get update", "&& apt-get install curl"}}.Render()

	// Assert
	assert.Equal(t, "RUN apt-get update \\\n  && apt-get install curl", result)
}
