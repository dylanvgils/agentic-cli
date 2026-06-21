package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintBasesField(t *testing.T) {
	t.Run("no bases shows none", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{}},
		}

		// Act
		err := printBasesField(&buf, layers)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "  bases: (none)\n", buf.String())
	})

	t.Run("base without version shows plain name", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}}}},
		}

		// Act
		err := printBasesField(&buf, layers)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "- java  [/project/.agenticrc.toml]")
	})

	t.Run("base with rc version shows at-version", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}, Versions: map[string]string{"java": "17"}}}},
		}

		// Act
		err := printBasesField(&buf, layers)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "- java@17  [/project/.agenticrc.toml]")
	})

	t.Run("innermost layer version wins", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{Path: "/home/.agenticrc.toml", RC: &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}, Versions: map[string]string{"java": "11"}}}},
			{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{Build: config.RCBuild{Versions: map[string]string{"java": "17"}}}},
		}

		// Act
		err := printBasesField(&buf, layers)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "- java@17  [/home/.agenticrc.toml]")
		assert.NotContains(t, out, "java@11")
	})

	t.Run("env var overrides rc version", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_JAVA_VERSION", "21")
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}, Versions: map[string]string{"java": "17"}}}},
		}

		// Act
		err := printBasesField(&buf, layers)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "- java@21  [/project/.agenticrc.toml]")
		assert.NotContains(t, out, "java@17")
	})

	t.Run("multiple layers show all entries with correct attribution", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{Path: "/home/.agenticrc.toml", RC: &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}}}},
			{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{Build: config.RCBuild{Bases: []string{"dotnet"}}}},
		}

		// Act
		err := printBasesField(&buf, layers)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "- java  [/home/.agenticrc.toml]")
		assert.Contains(t, out, "- dotnet  [/project/.agenticrc.toml]")
		javaIdx := bytes.Index(buf.Bytes(), []byte("- java"))
		dotnetIdx := bytes.Index(buf.Bytes(), []byte("- dotnet"))
		assert.Less(t, javaIdx, dotnetIdx)
	})
}

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
	get := func(rc *config.AgenticRC) string { return rc.Run.PidsLimit }

	t.Run("rc wins over env var", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_PIDS_LIMIT", "512")
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{Run: config.RCRun{PidsLimit: "100"}}},
		}

		// Act
		err := printScalarField(&buf, "pids_limit", "AGENTIC_PIDS_LIMIT", layers, get, "1024")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "  pids_limit: 100  [/project/.agenticrc.toml]\n", buf.String())
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

func TestPrintBoolField(t *testing.T) {
	get := func(rc *config.AgenticRC) *bool { return rc.Run.Proxy.Enabled }

	t.Run("no layer sets it shows default", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{}},
		}

		// Act
		err := printBoolField(&buf, "proxy.enabled", layers, get, false)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "  proxy.enabled: false  (default)\n", buf.String())
	})

	t.Run("layer sets true shows true with path", func(t *testing.T) {
		// Arrange
		enabled := true
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &enabled}}}},
		}

		// Act
		err := printBoolField(&buf, "proxy.enabled", layers, get, false)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "  proxy.enabled: true  [/project/.agenticrc.toml]\n", buf.String())
	})

	t.Run("innermost layer wins over outer layer", func(t *testing.T) {
		// Arrange
		outer, inner := true, false
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{Path: "/home/.agenticrc.toml", RC: &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &outer}}}},
			{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &inner}}}},
		}

		// Act
		err := printBoolField(&buf, "proxy.enabled", layers, get, false)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "  proxy.enabled: false  [/project/.agenticrc.toml]\n", buf.String())
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
		assert.Contains(t, out, "Project (.agenticrc.toml)")
		assert.Contains(t, out, "no .agenticrc.toml files found")
	})

	t.Run("single layer", func(t *testing.T) {
		// Arrange
		enabled := true
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{
				Path: "/project/.agenticrc.toml",
				RC: &config.AgenticRC{
					Namespace: "myproject",
					Build:     config.RCBuild{AptPackages: []string{"make"}, Bases: []string{"java"}, Versions: map[string]string{"java": "17"}},
					Run: config.RCRun{
						PidsLimit: "100", CPUs: "2", Memory: "4g",
						ExtraMounts: []string{"vol:/mnt"}, Secrets: []string{"tok:/run/s/t"},
						Proxy: config.RCProxy{Enabled: &enabled, AllowedHosts: []string{".github.com"}},
					},
				},
			},
		}

		// Act
		err := printProjectConfig(&buf, layers)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "Project (.agenticrc.toml, 1 file)")
		assert.Contains(t, out, "namespace: myproject  [/project/.agenticrc.toml]")
		assert.Contains(t, out, "pids_limit: 100  [/project/.agenticrc.toml]")
		assert.Contains(t, out, "cpus: 2  [/project/.agenticrc.toml]")
		assert.Contains(t, out, "memory: 4g  [/project/.agenticrc.toml]")
		assert.Contains(t, out, "- vol:/mnt  [/project/.agenticrc.toml]")
		assert.Contains(t, out, "- make  [/project/.agenticrc.toml]")
		assert.Contains(t, out, "- java@17  [/project/.agenticrc.toml]")
		assert.Contains(t, out, "- tok:/run/s/t  [/project/.agenticrc.toml]")
		assert.Contains(t, out, "proxy.enabled: true  [/project/.agenticrc.toml]")
		assert.Contains(t, out, "- .github.com  [/project/.agenticrc.toml]")
	})

	t.Run("multi layers source attribution", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{
				Path: "/home/.agenticrc.toml",
				RC: &config.AgenticRC{
					Build: config.RCBuild{AptPackages: []string{"make"}},
					Run:   config.RCRun{CPUs: "2", ExtraMounts: []string{"parent-vol:/mnt/p"}},
				},
			},
			{
				Path: "/project/.agenticrc.toml",
				RC: &config.AgenticRC{
					Build: config.RCBuild{AptPackages: []string{"gcc"}},
					Run:   config.RCRun{CPUs: "8", PidsLimit: "100", ExtraMounts: []string{"child-vol:/mnt/c"}},
				},
			},
		}

		// Act
		err := printProjectConfig(&buf, layers)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "Project (.agenticrc.toml, 2 files)")
		assert.Contains(t, out, "cpus: 8  [/project/.agenticrc.toml]")
		assert.Contains(t, out, "pids_limit: 100  [/project/.agenticrc.toml]")
		assert.Contains(t, out, "memory: 4g  (default)")
		assert.Contains(t, out, "- parent-vol:/mnt/p  [/home/.agenticrc.toml]")
		assert.Contains(t, out, "- child-vol:/mnt/c  [/project/.agenticrc.toml]")
		parentIdx := bytes.Index(buf.Bytes(), []byte("parent-vol"))
		childIdx := bytes.Index(buf.Bytes(), []byte("child-vol"))
		assert.Less(t, parentIdx, childIdx)
		assert.Contains(t, out, "- make  [/home/.agenticrc.toml]")
		assert.Contains(t, out, "- gcc  [/project/.agenticrc.toml]")
		makeIdx := bytes.Index(buf.Bytes(), []byte("- make"))
		gccIdx := bytes.Index(buf.Bytes(), []byte("- gcc"))
		assert.Less(t, makeIdx, gccIdx)
	})

	t.Run("no values shows defaults", func(t *testing.T) {
		// Arrange
		os.Unsetenv("AGENTIC_NAMESPACE")  //nolint:errcheck
		os.Unsetenv("AGENTIC_PIDS_LIMIT") //nolint:errcheck
		os.Unsetenv("AGENTIC_CPUS")       //nolint:errcheck
		os.Unsetenv("AGENTIC_MEMORY")     //nolint:errcheck
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{}},
		}

		// Act
		err := printProjectConfig(&buf, layers)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "namespace: agentic  (default)")
		assert.Contains(t, out, "pids_limit: 1024  (default)")
		assert.Contains(t, out, "cpus: 4  (default)")
		assert.Contains(t, out, "memory: 4g  (default)")
		assert.Contains(t, out, "apt_packages: (none)")
		assert.Contains(t, out, "bases: (none)")
		assert.Contains(t, out, "extra_mounts: (none)")
		assert.Contains(t, out, "secrets: (none)")
		assert.Contains(t, out, "proxy.enabled: false  (default)")
		assert.Contains(t, out, "proxy.allowed_hosts: (none)")
	})
}
