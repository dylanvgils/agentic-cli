package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNames(t *testing.T) {
	names := Names()

	t.Run("sorted", func(t *testing.T) {
		// Assert
		assert.Equal(t, []string{"claude", "copilot", "opencode"}, names)
	})

	t.Run("match config keys", func(t *testing.T) {
		// Assert
		assert.Len(t, names, len(Configs))
		for _, n := range names {
			assert.Contains(t, Configs, n)
		}
	})
}

func TestImageName(t *testing.T) {
	for _, tc := range []struct {
		name      string
		namespace string
		want      string
	}{
		{
			name:      "known tool with default namespace",
			namespace: "agentic",
			want:      "agentic-claude",
		},
		{
			name:      "known tool with custom namespace",
			namespace: "myproject",
			want:      "myproject-claude",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			image, err := ImageName("claude", tc.namespace)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, image)
		})
	}

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		_, err := ImageName("bogus", "agentic")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bogus")
	})
}

func TestParseImageName(t *testing.T) {
	for _, tc := range []struct {
		name          string
		image         string
		wantNamespace string
		wantTool      string
		wantOK        bool
	}{
		{
			name:          "default namespace and known tool",
			image:         "agentic-claude",
			wantNamespace: "agentic",
			wantTool:      "claude",
			wantOK:        true,
		},
		{
			name:          "custom namespace and known tool",
			image:         "myproject-copilot",
			wantNamespace: "myproject",
			wantTool:      "copilot",
			wantOK:        true,
		},
		{
			name:          "multi-segment namespace",
			image:         "my-long-project-opencode",
			wantNamespace: "my-long-project",
			wantTool:      "opencode",
			wantOK:        true,
		},
		{
			name:  "unknown tool returns false",
			image: "agentic-bogus",
		},
		{
			name:  "no dash returns false",
			image: "claude",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			namespace, tool, ok := ParseImageName(tc.image)

			// Assert
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantNamespace, namespace)
			assert.Equal(t, tc.wantTool, tool)
		})
	}
}

func TestRuntimeConfig_InstructionsHostPath(t *testing.T) {
	// Arrange
	c := RuntimeConfig{InstructionsFile: "example/data/NOTES.md"}

	// Act
	got := c.InstructionsHostPath("/home/test")

	// Assert
	assert.Equal(t, filepath.Join("/home/test", ToolsDirName, "example", "data", "NOTES.md"), got)
}

func TestRuntimeConfig_WriteInstructions(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	c := RuntimeConfig{InstructionsFile: "example/data/NOTES.md"}

	// Act
	err := c.WriteInstructions(dir, "# Environment\n")

	// Assert
	require.NoError(t, err)
	got, err := os.ReadFile(filepath.Join(dir, ToolsDirName, "example", "data", "NOTES.md"))
	require.NoError(t, err)
	assert.Contains(t, string(got), "\n# Environment\n")
}

func TestConfigs_stage(t *testing.T) {
	for _, name := range Names() {
		t.Run(name+" builds the tool stage as its own user", func(t *testing.T) {
			// Act
			stage := Configs[name].Build.Stage("prev")

			// Assert
			assert.Equal(t, "prev", stage.From.Image)
			assert.Equal(t, "tool", stage.From.As)
			result := renderStage(stage)
			assert.Contains(t, result, "useradd -l -u ${HOST_UID} -g ${HOST_GID} -m -s /bin/bash --non-unique "+name)
			assert.Contains(t, result, "TOOL_HOME=/home/"+name)
			assert.Contains(t, result, "agentic-version-"+name)
			assert.Contains(t, result, name+" --version")
		})
	}
}
