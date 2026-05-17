package cmd

import (
	"bytes"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- printGlobalConfig ---
func TestPrintGlobalConfig_Empty(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	cfg := &config.CliConfig{}

	// Act
	err := printGlobalConfig(&buf, "/home/user/.agentic", cfg)

	// Assert
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "Global (/home/user/.agentic/agentic.json)")
	assert.Contains(t, out, "trusted_dirs: (none)")
}

func TestPrintGlobalConfig_WithDirs(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	cfg := &config.CliConfig{TrustedDirs: []string{"/home/user/projects", "/home/user/work"}}

	// Act
	err := printGlobalConfig(&buf, "/home/user/.agentic", cfg)

	// Assert
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "trusted_dirs:")
	assert.Contains(t, out, "- /home/user/projects")
	assert.Contains(t, out, "- /home/user/work")
}

// --- printProjectConfig ---
func TestPrintProjectConfig_NoLayers(t *testing.T) {
	// Arrange
	var buf bytes.Buffer

	// Act
	err := printProjectConfig(&buf, nil)

	// Assert
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "Project (.agenticrc)")
	assert.Contains(t, out, "no .agenticrc files found")
}

func TestPrintProjectConfig_SingleLayer(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	layers := []config.RCLayer{
		{
			Path: "/project/.agenticrc",
			RC:   &config.AgenticRC{PidsLimit: "100", CPUs: "2", Memory: "4g", ExtraMounts: []string{"vol:/mnt"}, Secrets: []string{"tok=/run/s/t"}},
		},
	}

	// Act
	err := printProjectConfig(&buf, layers)

	// Assert
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "Project (.agenticrc, 1 file)")
	assert.Contains(t, out, "pids_limit: 100  [/project/.agenticrc]")
	assert.Contains(t, out, "cpus: 2  [/project/.agenticrc]")
	assert.Contains(t, out, "memory: 4g  [/project/.agenticrc]")
	assert.Contains(t, out, "- vol:/mnt  [/project/.agenticrc]")
	assert.Contains(t, out, "- tok=/run/s/t  [/project/.agenticrc]")
}

func TestPrintProjectConfig_MultiLayers_SourceAttribution(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	layers := []config.RCLayer{
		{
			Path: "/home/.agenticrc",
			RC:   &config.AgenticRC{CPUs: "2", ExtraMounts: []string{"parent-vol:/mnt/p"}},
		},
		{
			Path: "/project/.agenticrc",
			RC:   &config.AgenticRC{CPUs: "8", PidsLimit: "100", ExtraMounts: []string{"child-vol:/mnt/c"}},
		},
	}

	// Act
	err := printProjectConfig(&buf, layers)

	// Assert
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "Project (.agenticrc, 2 files)")
	// Innermost (child) wins for scalars
	assert.Contains(t, out, "cpus: 8  [/project/.agenticrc]")
	assert.Contains(t, out, "pids_limit: 100  [/project/.agenticrc]")
	assert.Contains(t, out, "memory: (not set)")
	// List entries tagged per-layer, outermost first
	assert.Contains(t, out, "- parent-vol:/mnt/p  [/home/.agenticrc]")
	assert.Contains(t, out, "- child-vol:/mnt/c  [/project/.agenticrc]")
	// Parent entry must appear before child entry
	parentIdx := bytes.Index(buf.Bytes(), []byte("parent-vol"))
	childIdx := bytes.Index(buf.Bytes(), []byte("child-vol"))
	assert.Less(t, parentIdx, childIdx)
}

func TestPrintProjectConfig_NoValues_ShowsNotSet(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	layers := []config.RCLayer{
		{Path: "/project/.agenticrc", RC: &config.AgenticRC{}},
	}

	// Act
	err := printProjectConfig(&buf, layers)

	// Assert
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "pids_limit: (not set)")
	assert.Contains(t, out, "cpus: (not set)")
	assert.Contains(t, out, "memory: (not set)")
	assert.Contains(t, out, "extra_mounts: (none)")
	assert.Contains(t, out, "secrets: (none)")
}
