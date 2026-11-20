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

func TestRun_renderBlocks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		blocks []Block
		want   string
	}{
		{
			name: "joins blocks with && and lines with a continuation",
			blocks: []Block{
				{Lines: []string{"apt-get update"}},
				{Lines: []string{"apt-get install curl", "wget"}},
				{Lines: []string{"rm -rf /var/lib/apt/lists/*"}},
			},
			want: "RUN apt-get update \\\n  && apt-get install curl \\\n  wget \\\n  && rm -rf /var/lib/apt/lists/*",
		},
		{
			name: "chain joins a block's lines with &&",
			blocks: []Block{
				{Lines: []string{"apt-get update"}},
				{Chain: true, Lines: []string{"apt-get install curl", "wget"}},
				{Lines: []string{"rm -rf /var/lib/apt/lists/*"}},
			},
			want: "RUN apt-get update \\\n  && apt-get install curl \\\n  && wget \\\n  && rm -rf /var/lib/apt/lists/*",
		},
		{
			name: "with comment",
			blocks: []Block{
				{Comment: "Update package list", Lines: []string{"apt-get update"}},
				{Comment: "Install packages", Lines: []string{"apt-get install curl"}},
			},
			want: "RUN \\\n  # Update package list\n  apt-get update \\\n  \\\n  # Install packages\n  && apt-get install curl",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := Run{Blocks: tc.blocks}.renderBlocks()

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestRun_Render(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  Run
		want string
	}{
		{
			name: "blocks win over command",
			run: Run{
				Command: "ignored",
				Blocks:  []Block{{Lines: []string{"apt-get update"}}},
			},
			want: "RUN apt-get update",
		},
		{
			name: "command without blocks",
			run:  Run{Command: "apt-get update"},
			want: "RUN apt-get update",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := tc.run.Render()

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}
