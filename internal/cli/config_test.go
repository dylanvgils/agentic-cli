package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_printGlobalConfig(t *testing.T) {
	state := &config.State{}

	t.Run("empty config", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		cfg := &config.CliConfig{}

		// Act
		err := printGlobalConfig(&buf, "/home/user/.agentic", cfg, state)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "Global (/home/user/.agentic/agentic.json)")
		assert.Contains(t, out, "registry: (not set)")
		assert.Contains(t, out, "trusted_dirs: (none)  [/home/user/.agentic/state.json]")
	})

	t.Run("with registry", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		cfg := &config.CliConfig{Registry: "myregistry.example.com"}

		// Act
		err := printGlobalConfig(&buf, "/home/user/.agentic", cfg, state)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "registry: myregistry.example.com")
	})

	t.Run("with docker context", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		cfg := &config.CliConfig{DockerContext: "prod"}

		// Act
		err := printGlobalConfig(&buf, "/home/user/.agentic", cfg, state)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "docker_context: prod")
	})

	t.Run("with dirs", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		trusted := &config.State{TrustedDirs: []string{"/home/user/projects", "/home/user/work"}}

		// Act
		err := printGlobalConfig(&buf, "/home/user/.agentic", &config.CliConfig{}, trusted)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "trusted_dirs:")
		assert.Contains(t, out, "- /home/user/projects")
		assert.Contains(t, out, "- /home/user/work")
	})
}

func Test_printProjectConfig(t *testing.T) {
	t.Run("no layers", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer

		// Act
		err := printProjectConfig(&buf, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "Project (.agenticrc.toml)\n  (no .agenticrc.toml files found)\n", buf.String())
	})

	t.Run("groups keys under table headers", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		layers := []config.RCLayer{{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{}}}

		// Act
		err := printProjectConfig(&buf, layers)

		// Assert
		require.NoError(t, err)
		want := `Project (.agenticrc.toml, 1 file)
  namespace: agentic  (default)
  docker_context: (not set)
  marketplaces: (none)

  [build]
    bases: (none)
    apt_packages: (none)
    custom_installs: (none)

  [run]
    pids_limit: 1024  (default)
    cpus: 4  (default)
    memory: 4g  (default)
    mounts: (none)
    secrets: (none)
    env: (none)
    args: (none)
    check_updates: true  (default)

  [run.instructions]
    enabled: true  (default)
    custom: (none)

  [run.proxy]
    enabled: false  (default)
    mode: enforce  (default)
    allowed_hosts: (none)
    credentials: (none)

  [run.dind]
    enabled: false  (default)
    pids_limit: 1024  (default)
    cpus: 4  (default)
    memory: 4g  (default)
`
		assert.Equal(t, want, buf.String())
	})

	t.Run("multi layers source attribution", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		layers := []config.RCLayer{
			{
				Path: "/home/.agenticrc.toml",
				RC: &config.AgenticRC{
					Build: config.RCBuild{AptPackages: []string{"make"}},
					Run:   config.RCRun{RCLimits: config.RCLimits{CPUs: "2"}, Mounts: []string{"parent-vol:/mnt/p"}},
				},
			},
			{
				Path: "/project/.agenticrc.toml",
				RC: &config.AgenticRC{
					Build: config.RCBuild{AptPackages: []string{"gcc"}},
					Run:   config.RCRun{RCLimits: config.RCLimits{CPUs: "8"}, Mounts: []string{"child-vol:/mnt/c"}},
				},
			},
		}

		// Act
		err := printProjectConfig(&buf, layers)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Contains(t, out, "Project (.agenticrc.toml, 2 files)")
		assert.Contains(t, out, "    cpus: 8  [/project/.agenticrc.toml]")
		assert.Contains(t, out, "      - make  [/home/.agenticrc.toml]\n      - gcc  [/project/.agenticrc.toml]\n")
		assert.Contains(t, out, "      - parent-vol:/mnt/p  [/home/.agenticrc.toml]\n      - child-vol:/mnt/c  [/project/.agenticrc.toml]\n")
	})
}

func Test_projectSections(t *testing.T) {
	t.Run("dind limits inherit tool values unless set", func(t *testing.T) {
		// Arrange
		layers := []config.RCLayer{{
			Path: "/project/.agenticrc.toml",
			RC: &config.AgenticRC{Run: config.RCRun{
				RCLimits: config.RCLimits{CPUs: "8", PidsLimit: "100"},
				Dind:     config.RCDind{RCLimits: config.RCLimits{Memory: "16g"}},
			}},
		}}

		// Act
		sections := projectSections(layers)

		// Assert
		dind := sections[len(sections)-1]
		assert.Equal(t, "run.dind", dind.header)
		assert.Equal(t, []field{
			{key: "enabled", entries: []entry{{value: "false"}}},
			{key: "pids_limit", entries: []entry{{value: "100"}}},
			{key: "cpus", entries: []entry{{value: "8"}}},
			{key: "memory", entries: []entry{{value: "16g", source: "/project/.agenticrc.toml"}}},
		}, dind.fields)
	})

	t.Run("bases show the innermost version", func(t *testing.T) {
		// Arrange
		layers := []config.RCLayer{
			{Path: "/home/.agenticrc.toml", RC: &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java", "go"}, Versions: map[string]string{"java": "11"}}}},
			{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{Build: config.RCBuild{Versions: map[string]string{"java": "17"}}}},
		}

		// Act
		sections := projectSections(layers)

		// Assert
		bases := sections[1].fields[0]
		assert.Equal(t, []entry{
			{value: "java@17", source: "/home/.agenticrc.toml"},
			{value: "go", source: "/home/.agenticrc.toml"},
		}, bases.entries)
	})
}

func Test_proxyEnabledDefault(t *testing.T) {
	t.Run("off without monitor mode or credentials", func(t *testing.T) {
		// Arrange
		layers := []config.RCLayer{{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{}}}

		// Act
		enabled := proxyEnabledDefault(layers)

		// Assert
		assert.False(t, enabled)
	})

	t.Run("on when the innermost mode is monitor", func(t *testing.T) {
		// Arrange
		layers := []config.RCLayer{
			{Path: "/.agenticrc.toml", RC: &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Mode: config.ModeEnforce}}}},
			{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Mode: config.ModeMonitor}}}},
		}

		// Act
		enabled := proxyEnabledDefault(layers)

		// Assert
		assert.True(t, enabled)
	})

	t.Run("on when any layer has credentials", func(t *testing.T) {
		// Arrange
		creds := []config.RCCredential{{Preset: "github", Secret: "~/.config/test-secret"}}
		layers := []config.RCLayer{
			{Path: "/.agenticrc.toml", RC: &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Credentials: creds}}}},
			{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{}},
		}

		// Act
		enabled := proxyEnabledDefault(layers)

		// Assert
		assert.True(t, enabled)
	})
}

func Test_limitFields(t *testing.T) {
	// Arrange
	layers := []config.RCLayer{{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{Run: config.RCRun{RCLimits: config.RCLimits{CPUs: "8"}}}}}
	get := func(rc *config.AgenticRC) config.RCLimits { return rc.Run.RCLimits }

	// Act
	fields := limitFields(layers, get, sandbox.DefaultLimits)

	// Assert
	assert.Equal(t, []field{
		{key: "pids_limit", entries: []entry{{value: "1024"}}},
		{key: "cpus", entries: []entry{{value: "8", source: "/project/.agenticrc.toml"}}},
		{key: "memory", entries: []entry{{value: "4g"}}},
	}, fields)
}

func Test_scalarField(t *testing.T) {
	namespace := func(rc *config.AgenticRC) string { return rc.Namespace }

	t.Run("innermost layer wins", func(t *testing.T) {
		// Arrange
		layers := []config.RCLayer{
			{Path: "/home/.agenticrc.toml", RC: &config.AgenticRC{Namespace: "outer"}},
			{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{Namespace: "inner"}},
		}

		// Act
		f := scalarField(layers, "namespace", namespace, "agentic")

		// Assert
		assert.Equal(t, field{key: "namespace", entries: []entry{{value: "inner", source: "/project/.agenticrc.toml"}}}, f)
	})

	t.Run("default when no layer sets it", func(t *testing.T) {
		// Arrange
		layers := []config.RCLayer{{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{}}}

		// Act
		f := scalarField(layers, "namespace", namespace, "agentic")

		// Assert
		assert.Equal(t, field{key: "namespace", entries: []entry{{value: "agentic"}}}, f)
	})

	t.Run("no entry without value or default", func(t *testing.T) {
		// Arrange
		layers := []config.RCLayer{{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{}}}

		// Act
		f := scalarField(layers, "namespace", namespace, "")

		// Assert
		assert.Equal(t, field{key: "namespace"}, f)
	})
}

func Test_boolField(t *testing.T) {
	enabled := func(rc *config.AgenticRC) *bool { return rc.Run.Proxy.Enabled }

	t.Run("innermost layer that sets it wins", func(t *testing.T) {
		// Arrange
		on, off := true, false
		layers := []config.RCLayer{
			{Path: "/home/.agenticrc.toml", RC: &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &on}}}},
			{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &off}}}},
		}

		// Act
		f := boolField(layers, "enabled", enabled, true)

		// Assert
		assert.Equal(t, field{key: "enabled", entries: []entry{{value: "false", source: "/project/.agenticrc.toml"}}}, f)
	})

	t.Run("default when no layer sets it", func(t *testing.T) {
		// Arrange
		layers := []config.RCLayer{{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{}}}

		// Act
		f := boolField(layers, "enabled", enabled, true)

		// Assert
		assert.Equal(t, field{key: "enabled", entries: []entry{{value: "true"}}}, f)
	})
}

func Test_listField(t *testing.T) {
	// Arrange
	layers := []config.RCLayer{
		{Path: "/home/.agenticrc.toml", RC: &config.AgenticRC{Run: config.RCRun{Secrets: []string{"a:/run/a"}}}},
		{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{Run: config.RCRun{Secrets: []string{"b:/run/b"}}}},
	}
	secrets := func(rc *config.AgenticRC) []string { return rc.Run.Secrets }

	// Act
	f := listField(layers, "secrets", secrets)

	// Assert
	assert.Equal(t, field{key: "secrets", list: true, entries: []entry{
		{value: "a:/run/a", source: "/home/.agenticrc.toml"},
		{value: "b:/run/b", source: "/project/.agenticrc.toml"},
	}}, f)
}

func Test_innermost(t *testing.T) {
	layers := []config.RCLayer{
		{Path: "/home/.agenticrc.toml", RC: &config.AgenticRC{Namespace: "outer"}},
		{Path: "/project/.agenticrc.toml", RC: &config.AgenticRC{}},
	}
	namespace := func(rc *config.AgenticRC) string { return rc.Namespace }

	t.Run("skips layers where it is unset", func(t *testing.T) {
		// Act
		v, path, ok := innermost(layers, namespace, func(v string) bool { return v != "" })

		// Assert
		assert.True(t, ok)
		assert.Equal(t, "outer", v)
		assert.Equal(t, "/home/.agenticrc.toml", path)
	})

	t.Run("not found when no layer sets it", func(t *testing.T) {
		// Act
		_, _, ok := innermost(layers, namespace, func(string) bool { return false })

		// Assert
		assert.False(t, ok)
	})
}

func Test_writeSection(t *testing.T) {
	fields := []field{{key: "enabled", entries: []entry{{value: "true"}}}}

	t.Run("top level has no header", func(t *testing.T) {
		// Arrange
		var b strings.Builder

		// Act
		writeSection(&b, section{fields: fields})

		// Assert
		assert.Equal(t, "  enabled: true  (default)\n", b.String())
	})

	t.Run("table indents its fields under the header", func(t *testing.T) {
		// Arrange
		var b strings.Builder

		// Act
		writeSection(&b, section{header: "run.proxy", fields: fields})

		// Assert
		assert.Equal(t, "\n  [run.proxy]\n    enabled: true  (default)\n", b.String())
	})
}

func Test_writeField(t *testing.T) {
	tests := []struct {
		name string
		f    field
		want string
	}{
		{"scalar from a layer", field{key: "cpus", entries: []entry{{value: "8", source: "/p/.agenticrc.toml"}}}, "  cpus: 8  [/p/.agenticrc.toml]\n"},
		{"scalar default", field{key: "cpus", entries: []entry{{value: "4"}}}, "  cpus: 4  (default)\n"},
		{"scalar not set", field{key: "docker_context"}, "  docker_context: (not set)\n"},
		{"empty list", field{key: "secrets", list: true}, "  secrets: (none)\n"},
		{"list entries", field{key: "secrets", list: true, entries: []entry{{value: "a", source: "/p/.agenticrc.toml"}}}, "  secrets:\n    - a  [/p/.agenticrc.toml]\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var b strings.Builder

			// Act
			writeField(&b, tt.f, "  ")

			// Assert
			assert.Equal(t, tt.want, b.String())
		})
	}
}

func Test_customInstallNames(t *testing.T) {
	// Arrange
	rc := &config.AgenticRC{}
	rc.Build.CustomInstalls = []config.RCCustomInstall{{Name: "golangci-lint"}, {Name: "terraform"}}

	// Act
	result := customInstallNames(rc)

	// Assert
	assert.Equal(t, []string{"golangci-lint", "terraform"}, result)
}

func Test_envKeys(t *testing.T) {
	// Arrange
	rc := &config.AgenticRC{Run: config.RCRun{Env: []string{"TOKEN=test-secret", "HOME_DIR"}}}

	// Act
	result := envKeys(rc)

	// Assert
	assert.Equal(t, []string{"TOKEN", "HOME_DIR"}, result)
}

func Test_toolArgs(t *testing.T) {
	// Arrange
	rc := &config.AgenticRC{Run: config.RCRun{Args: map[string][]string{
		"copilot": {"--disable-client-telemetry", "--example"},
		"claude":  {"--verbose"},
	}}}

	// Act
	result := toolArgs(rc)

	// Assert
	assert.Equal(t, []string{"claude: --verbose", "copilot: --disable-client-telemetry --example"}, result)
}

func Test_credentialNames(t *testing.T) {
	// Arrange
	rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Credentials: []config.RCCredential{
		{Preset: "anthropic", Secret: "~/.secrets/a"},
		{Hosts: []string{"api.example.test", "upload.example.test"}, Header: "Authorization", Secret: "~/.secrets/b"},
	}}}}

	// Act
	result := credentialNames(rc)

	// Assert
	assert.Equal(t, []string{"anthropic", "api.example.test, upload.example.test"}, result)
}

func Test_marketplaceNames(t *testing.T) {
	// Arrange
	rc := &config.AgenticRC{Marketplaces: []config.RCMarketplace{
		{Name: "acme", URL: "https://git.example.test/acme.git"},
		{Name: "team", URL: "https://git.example.test/team.git", Tools: []string{"claude", "copilot"}},
	}}

	// Act
	result := marketplaceNames(rc)

	// Assert
	assert.Equal(t, []string{
		"acme (https://git.example.test/acme.git)",
		"team (https://git.example.test/team.git) tools: claude, copilot",
	}, result)
}

func Test_customInstructions(t *testing.T) {
	tests := []struct {
		name   string
		custom string
		want   []string
	}{
		{"unset", "", nil},
		{"one line", "Use make.\n", []string{"(1 line)"}},
		{"several lines", "Use make.\nRun lint.\n", []string{"(2 lines)"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			rc := &config.AgenticRC{Run: config.RCRun{Instructions: config.RCInstructions{Custom: tt.custom}}}

			// Act
			result := customInstructions(rc)

			// Assert
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_showConfig(t *testing.T) {
	// Arrange
	withTempToolHome(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".agenticrc.toml"), []byte("root = true\nnamespace = \"work\"\n"), 0o600))
	t.Chdir(dir)
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	// Act
	err := showConfig(cmd, nil)

	// Assert - the global section, then the project layers from the working dir
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "Global (")
	assert.Contains(t, out, "Project (.agenticrc.toml, 1 file)")
	assert.Contains(t, out, "work")
	assert.Less(t, strings.Index(out, "Global ("), strings.Index(out, "Project ("))
}
