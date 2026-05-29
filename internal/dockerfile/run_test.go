package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun_renderCommand(t *testing.T) {
	// Act
	result := Run{Command: "apt-get update"}.renderCommand()

	// Assert
	assert.Equal(t, "RUN apt-get update", result)
}

func TestRun_renderLines(t *testing.T) {
	// Act
	result := Run{Lines: []string{"apt-get update", "&& apt-get install curl"}}.renderLines()

	// Assert
	assert.Equal(t, "RUN apt-get update \\\n  && apt-get install curl", result)
}

func TestRun_renderBlocks(t *testing.T) {
	t.Run("no comment", func(t *testing.T) {
		// Arrange
		run := Run{Blocks: []Block{
			{Lines: []string{"apt-get update"}},
			{Lines: []string{"apt-get install curl", "wget"}},
			{Lines: []string{"rm -rf /var/lib/apt/lists/*"}},
		}}

		// Act
		result := run.renderBlocks()

		// Assert
		assert.Equal(t, "RUN apt-get update \\\n  && apt-get install curl \\\n  wget \\\n  && rm -rf /var/lib/apt/lists/*", result)
	})

	t.Run("with comment", func(t *testing.T) {
		// Arrange
		run := Run{Blocks: []Block{
			{Comment: "Update package list", Lines: []string{"apt-get update"}},
			{Comment: "Install packages", Lines: []string{"apt-get install curl"}},
		}}

		// Act
		result := run.renderBlocks()

		// Assert
		assert.Equal(t, "RUN \\\n  # Update package list\n  apt-get update \\\n  \\\n  # Install packages\n  && apt-get install curl", result)
	})
}

func TestRun_render_dispatchesCorrectly(t *testing.T) {
	tests := []struct {
		name     string
		run      Run
		expected string
	}{
		{
			name:     "dispatches to renderBlocks",
			run:      Run{Blocks: []Block{{Lines: []string{"apt-get update"}}}},
			expected: Run{Blocks: []Block{{Lines: []string{"apt-get update"}}}}.renderBlocks(),
		},
		{
			name:     "dispatches to renderLines",
			run:      Run{Lines: []string{"apt-get update"}},
			expected: Run{Lines: []string{"apt-get update"}}.renderLines(),
		},
		{
			name:     "dispatches to renderCommand",
			run:      Run{Command: "apt-get update"},
			expected: Run{Command: "apt-get update"}.renderCommand(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := tt.run.Render()

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}
