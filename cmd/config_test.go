package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintGlobalConfig(t *testing.T) {
	t.Run("empty config", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		cfg := &config.CliConfig{}

		// Act
		err := printGlobalConfig(&buf, "/home/user/.agentic", cfg)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "Global (/home/user/.agentic/agentic.json)")
		assert.Contains(t, out, "registry: (not set)")
		assert.Contains(t, out, "trusted_dirs: (none)")
	})

	t.Run("with registry", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		cfg := &config.CliConfig{Registry: "myregistry.example.com"}

		// Act
		err := printGlobalConfig(&buf, "/home/user/.agentic", cfg)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "registry: myregistry.example.com")
		assert.NotContains(t, out, "(not set)")
	})

	t.Run("with dirs", func(t *testing.T) {
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
	})
}

func TestPrintScalarField(t *testing.T) {
	get := func(rc *config.AgenticRC) string { return rc.PidsLimit }

	t.Run("rc wins over env var", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_PIDS_LIMIT", "512")
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{Path: "/project/.agenticrc", RC: &config.AgenticRC{PidsLimit: "100"}},
		}

		// Act
		err := printScalarField(&buf, "pids_limit", "AGENTIC_PIDS_LIMIT", layers, get, "1024")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "  pids_limit: 100  [/project/.agenticrc]\n", buf.String())
	})

	t.Run("not set shown when no env, rc, or default", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer

		// Act
		err := printScalarField(&buf, "pids_limit", "", nil, get, "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "  pids_limit: (not set)\n", buf.String())
	})
}

func TestPrintProjectConfig(t *testing.T) {
	t.Run("no layers", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer

		// Act
		err := printProjectConfig(&buf, nil)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "Project (.agenticrc)")
		assert.Contains(t, out, "no .agenticrc files found")
	})

	t.Run("single layer", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{
				Path: "/project/.agenticrc",
				RC:   &config.AgenticRC{PidsLimit: "100", CPUs: "2", Memory: "4g", Prefix: "myproject", ExtraMounts: []string{"vol:/mnt"}, AptPackages: []string{"make"}, Secrets: []string{"tok:/run/s/t"}},
			},
		}

		// Act
		err := printProjectConfig(&buf, layers)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "Project (.agenticrc, 1 file)")
		assert.Contains(t, out, "prefix: myproject  [/project/.agenticrc]")
		assert.Contains(t, out, "pids_limit: 100  [/project/.agenticrc]")
		assert.Contains(t, out, "cpus: 2  [/project/.agenticrc]")
		assert.Contains(t, out, "memory: 4g  [/project/.agenticrc]")
		assert.Contains(t, out, "- vol:/mnt  [/project/.agenticrc]")
		assert.Contains(t, out, "- make  [/project/.agenticrc]")
		assert.Contains(t, out, "- tok:/run/s/t  [/project/.agenticrc]")
	})

	t.Run("multi layers source attribution", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{
				Path: "/home/.agenticrc",
				RC:   &config.AgenticRC{CPUs: "2", ExtraMounts: []string{"parent-vol:/mnt/p"}, AptPackages: []string{"make"}},
			},
			{
				Path: "/project/.agenticrc",
				RC:   &config.AgenticRC{CPUs: "8", PidsLimit: "100", ExtraMounts: []string{"child-vol:/mnt/c"}, AptPackages: []string{"gcc"}},
			},
		}

		// Act
		err := printProjectConfig(&buf, layers)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "Project (.agenticrc, 2 files)")
		assert.Contains(t, out, "cpus: 8  [/project/.agenticrc]")
		assert.Contains(t, out, "pids_limit: 100  [/project/.agenticrc]")
		assert.Contains(t, out, "memory: 4g  (default)")
		assert.Contains(t, out, "- parent-vol:/mnt/p  [/home/.agenticrc]")
		assert.Contains(t, out, "- child-vol:/mnt/c  [/project/.agenticrc]")
		parentIdx := bytes.Index(buf.Bytes(), []byte("parent-vol"))
		childIdx := bytes.Index(buf.Bytes(), []byte("child-vol"))
		assert.Less(t, parentIdx, childIdx)
		assert.Contains(t, out, "- make  [/home/.agenticrc]")
		assert.Contains(t, out, "- gcc  [/project/.agenticrc]")
		makeIdx := bytes.Index(buf.Bytes(), []byte("- make"))
		gccIdx := bytes.Index(buf.Bytes(), []byte("- gcc"))
		assert.Less(t, makeIdx, gccIdx)
	})

	t.Run("no values shows defaults", func(t *testing.T) {
		// Arrange
		os.Unsetenv("AGENTIC_PREFIX")    //nolint:errcheck
		os.Unsetenv("AGENTIC_PIDS_LIMIT") //nolint:errcheck
		os.Unsetenv("AGENTIC_CPUS")       //nolint:errcheck
		os.Unsetenv("AGENTIC_MEMORY")     //nolint:errcheck
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{Path: "/project/.agenticrc", RC: &config.AgenticRC{}},
		}

		// Act
		err := printProjectConfig(&buf, layers)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "prefix: agentic  (default)")
		assert.Contains(t, out, "pids_limit: 1024  (default)")
		assert.Contains(t, out, "cpus: 4  (default)")
		assert.Contains(t, out, "memory: 4g  (default)")
		assert.Contains(t, out, "extra_mounts: (none)")
		assert.Contains(t, out, "apt_packages: (none)")
		assert.Contains(t, out, "secrets: (none)")
	})
}
