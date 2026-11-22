package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config/rc"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_printGlobalConfig(t *testing.T) {
	home := filepath.Join("/home", "user", ".agentic")
	configFile := filepath.Join(home, "agentic.json")
	stateFile := filepath.Join(home, "state.json")

	for _, tc := range []struct {
		name  string
		cfg   *rc.CliConfig
		state *rc.State
		want  string
	}{
		{
			name:  "empty config",
			cfg:   &rc.CliConfig{},
			state: &rc.State{},
			want: "Global (" + configFile + ")\n" +
				"  registry: (not set)\n" +
				"  docker_context: (not set)\n" +
				"  trusted_dirs: (none)  [" + stateFile + "]\n",
		},
		{
			name:  "every setting set",
			cfg:   &rc.CliConfig{Registry: "myregistry.example.com", DockerContext: "prod"},
			state: &rc.State{TrustedDirs: []string{"/home/user/projects", "/home/user/work"}},
			want: "Global (" + configFile + ")\n" +
				"  registry: myregistry.example.com\n" +
				"  docker_context: prod\n" +
				"  trusted_dirs:  [" + stateFile + "]\n" +
				"    - /home/user/projects\n" +
				"    - /home/user/work\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			buf := iostreamstest.StubOut(t)

			// Act
			printGlobalConfig(home, tc.cfg, tc.state)

			// Assert
			assert.Equal(t, tc.want, buf.String())
		})
	}
}

func Test_printProjectConfig(t *testing.T) {
	t.Run("no layers", func(t *testing.T) {
		// Arrange
		buf := iostreamstest.StubOut(t)

		// Act
		printProjectConfig(nil)

		// Assert
		assert.Equal(t, "Project (.agenticrc.toml)\n  (no .agenticrc.toml files found)\n", buf.String())
	})

	t.Run("groups keys under table headers", func(t *testing.T) {
		// Arrange
		buf := iostreamstest.StubOut(t)
		layers := []rc.RCLayer{{Path: "/project/.agenticrc.toml", RC: &rc.AgenticRC{}}}

		// Act
		printProjectConfig(layers)

		// Assert
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
    allowed_paths: (none)
    credentials: (none)

  [run.dind]
    enabled: false  (default)
    pids_limit: 1024  (default)
    cpus: 4  (default)
    memory: 4g  (default)
`
		assert.Equal(t, want, buf.String())
	})

	t.Run("multi layers name the file count", func(t *testing.T) {
		// Arrange
		buf := iostreamstest.StubOut(t)
		layers := []rc.RCLayer{
			{Path: "/home/.agenticrc.toml", RC: &rc.AgenticRC{}},
			{Path: "/project/.agenticrc.toml", RC: &rc.AgenticRC{}},
		}

		// Act
		printProjectConfig(layers)

		// Assert
		assert.Contains(t, buf.String(), "Project (.agenticrc.toml, 2 files)")
	})
}

func Test_projectSections(t *testing.T) {
	t.Run("dind limits inherit tool values unless set", func(t *testing.T) {
		// Arrange
		layers := []rc.RCLayer{{
			Path: "/project/.agenticrc.toml",
			RC: &rc.AgenticRC{Run: rc.RCRun{
				RCLimits: rc.RCLimits{CPUs: "8", PidsLimit: "100"},
				Dind:     rc.RCDind{RCLimits: rc.RCLimits{Memory: "16g"}},
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
		layers := []rc.RCLayer{
			{Path: "/home/.agenticrc.toml", RC: &rc.AgenticRC{Build: rc.RCBuild{Bases: []string{"java", "go"}, Versions: map[string]string{"java": "11"}}}},
			{Path: "/project/.agenticrc.toml", RC: &rc.AgenticRC{Build: rc.RCBuild{Versions: map[string]string{"java": "17"}}}},
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
	for _, tc := range []struct {
		name  string
		outer rc.RCProxy
		inner rc.RCProxy
		want  bool
	}{
		{name: "off without monitor mode or credentials"},
		{name: "on when the innermost mode is monitor", outer: rc.RCProxy{Mode: rc.ModeEnforce}, inner: rc.RCProxy{Mode: rc.ModeMonitor}, want: true},
		{
			name:  "on when any layer has credentials",
			outer: rc.RCProxy{Credentials: []rc.RCCredential{{Preset: "github", Secret: "~/.config/test-secret"}}},
			want:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			layers := []rc.RCLayer{
				{Path: "/.agenticrc.toml", RC: &rc.AgenticRC{Run: rc.RCRun{Proxy: tc.outer}}},
				{Path: "/project/.agenticrc.toml", RC: &rc.AgenticRC{Run: rc.RCRun{Proxy: tc.inner}}},
			}

			// Act
			enabled := proxyEnabledDefault(layers)

			// Assert
			assert.Equal(t, tc.want, enabled)
		})
	}
}

func Test_scalarField(t *testing.T) {
	namespace := func(agenticRC *rc.AgenticRC) string { return agenticRC.Namespace }

	for _, tc := range []struct {
		name  string
		outer string
		inner string
		def   string
		want  field
	}{
		{
			name:  "innermost layer wins",
			outer: "outer",
			inner: "inner",
			def:   "agentic",
			want:  field{key: "namespace", entries: []entry{{value: "inner", source: "/project/.agenticrc.toml"}}},
		},
		{name: "default when no layer sets it", def: "agentic", want: field{key: "namespace", entries: []entry{{value: "agentic"}}}},
		{name: "no entry without value or default", want: field{key: "namespace"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			layers := []rc.RCLayer{
				{Path: "/home/.agenticrc.toml", RC: &rc.AgenticRC{Namespace: tc.outer}},
				{Path: "/project/.agenticrc.toml", RC: &rc.AgenticRC{Namespace: tc.inner}},
			}

			// Act
			f := scalarField(layers, "namespace", namespace, tc.def)

			// Assert
			assert.Equal(t, tc.want, f)
		})
	}
}

func Test_boolField(t *testing.T) {
	enabled := func(agenticRC *rc.AgenticRC) *bool { return agenticRC.Run.Proxy.Enabled }
	on, off := true, false

	for _, tc := range []struct {
		name   string
		layers []rc.RCLayer
		want   field
	}{
		{
			name: "innermost layer that sets it wins",
			layers: []rc.RCLayer{
				{Path: "/home/.agenticrc.toml", RC: &rc.AgenticRC{Run: rc.RCRun{Proxy: rc.RCProxy{Enabled: &on}}}},
				{Path: "/project/.agenticrc.toml", RC: &rc.AgenticRC{Run: rc.RCRun{Proxy: rc.RCProxy{Enabled: &off}}}},
			},
			want: field{key: "enabled", entries: []entry{{value: "false", source: "/project/.agenticrc.toml"}}},
		},
		{
			name:   "default when no layer sets it",
			layers: []rc.RCLayer{{Path: "/project/.agenticrc.toml", RC: &rc.AgenticRC{}}},
			want:   field{key: "enabled", entries: []entry{{value: "true"}}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			f := boolField(tc.layers, "enabled", enabled, true)

			// Assert
			assert.Equal(t, tc.want, f)
		})
	}
}

func Test_listField(t *testing.T) {
	// Arrange
	layers := []rc.RCLayer{
		{Path: "/home/.agenticrc.toml", RC: &rc.AgenticRC{Run: rc.RCRun{Mounts: []string{"a:/a"}}}},
		{Path: "/project/.agenticrc.toml", RC: &rc.AgenticRC{Run: rc.RCRun{Mounts: []string{"b:/b"}}}},
	}
	mounts := func(agenticRC *rc.AgenticRC) []string { return agenticRC.Run.Mounts }

	// Act
	f := listField(layers, "mounts", mounts)

	// Assert
	assert.Equal(t, field{key: "mounts", list: true, entries: []entry{
		{value: "a:/a", source: "/home/.agenticrc.toml"},
		{value: "b:/b", source: "/project/.agenticrc.toml"},
	}}, f)
}

func Test_writeSection(t *testing.T) {
	fields := []field{{key: "enabled", entries: []entry{{value: "true"}}}}

	for _, tc := range []struct {
		name    string
		section section
		want    string
	}{
		{
			name:    "top level has no header",
			section: section{fields: fields},
			want:    "  enabled: true  (default)\n",
		},
		{
			name:    "table indents its fields under the header",
			section: section{header: "run.proxy", fields: fields},
			want:    "\n  [run.proxy]\n    enabled: true  (default)\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			var b strings.Builder

			// Act
			writeSection(&b, tc.section)

			// Assert
			assert.Equal(t, tc.want, b.String())
		})
	}
}

func Test_writeField(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    field
		want string
	}{
		{
			name: "scalar from a layer",
			f:    field{key: "cpus", entries: []entry{{value: "8", source: "/p/.agenticrc.toml"}}},
			want: "  cpus: 8  [/p/.agenticrc.toml]\n",
		},
		{
			name: "scalar default",
			f:    field{key: "cpus", entries: []entry{{value: "4"}}},
			want: "  cpus: 4  (default)\n",
		},
		{
			name: "scalar not set",
			f:    field{key: "docker_context"},
			want: "  docker_context: (not set)\n",
		},
		{
			name: "empty list",
			f:    field{key: "mounts", list: true},
			want: "  mounts: (none)\n",
		},
		{
			name: "list entries",
			f:    field{key: "mounts", list: true, entries: []entry{{value: "a", source: "/p/.agenticrc.toml"}}},
			want: "  mounts:\n    - a  [/p/.agenticrc.toml]\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			var b strings.Builder

			// Act
			writeField(&b, tc.f, "  ")

			// Assert
			assert.Equal(t, tc.want, b.String())
		})
	}
}

func Test_customInstallNames(t *testing.T) {
	// Arrange
	agenticRC := &rc.AgenticRC{}
	agenticRC.Build.CustomInstalls = []rc.RCCustomInstall{{Name: "golangci-lint"}, {Name: "terraform"}}

	// Act
	result := customInstallNames(agenticRC)

	// Assert
	assert.Equal(t, []string{"golangci-lint", "terraform"}, result)
}

func Test_envKeys(t *testing.T) {
	// Arrange
	agenticRC := &rc.AgenticRC{Run: rc.RCRun{Env: []string{"TOKEN=test-secret", "HOME_DIR"}}}

	// Act
	result := envKeys(agenticRC)

	// Assert
	assert.Equal(t, []string{"TOKEN", "HOME_DIR"}, result)
}

func Test_toolArgs(t *testing.T) {
	// Arrange
	agenticRC := &rc.AgenticRC{Run: rc.RCRun{Args: map[string][]string{
		"copilot": {"--disable-client-telemetry", "--example"},
		"claude":  {"--verbose"},
	}}}

	// Act
	result := toolArgs(agenticRC)

	// Assert
	assert.Equal(t, []string{"claude: --verbose", "copilot: --disable-client-telemetry --example"}, result)
}

func Test_allowedPaths(t *testing.T) {
	// Arrange
	agenticRC := &rc.AgenticRC{Run: rc.RCRun{Proxy: rc.RCProxy{AllowedPaths: []rc.RCAllowedPath{
		{Host: "raw.example.test", Paths: []string{"/org/repo/**", "/org/*/README.md"}},
	}}}}

	// Act
	result := allowedPaths(agenticRC)

	// Assert
	assert.Equal(t, []string{"raw.example.test /org/repo/**, /org/*/README.md"}, result)
}

func Test_credentialNames(t *testing.T) {
	// Arrange
	agenticRC := &rc.AgenticRC{Run: rc.RCRun{Proxy: rc.RCProxy{Credentials: []rc.RCCredential{
		{Preset: "anthropic", Secret: "~/.secrets/a"},
		{Hosts: []string{"api.example.test", "upload.example.test"}, Header: "Authorization", Secret: "~/.secrets/b"},
	}}}}

	// Act
	result := credentialNames(agenticRC)

	// Assert
	assert.Equal(t, []string{"anthropic", "api.example.test, upload.example.test"}, result)
}

func Test_marketplaceNames(t *testing.T) {
	// Arrange
	agenticRC := &rc.AgenticRC{Marketplaces: []rc.RCMarketplace{
		{Name: "acme", URL: "https://git.example.test/acme.git"},
		{Name: "team", URL: "https://git.example.test/team.git", Tools: []string{"claude", "copilot"}},
	}}

	// Act
	result := marketplaceNames(agenticRC)

	// Assert
	assert.Equal(t, []string{
		"acme (https://git.example.test/acme.git)",
		"team (https://git.example.test/team.git) tools: claude, copilot",
	}, result)
}

func Test_customInstructions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		custom string
		want   []string
	}{
		{
			name: "unset",
		},
		{
			name:   "one line",
			custom: "Use make.\n",
			want:   []string{"(1 line)"},
		},
		{
			name:   "several lines",
			custom: "Use make.\nRun lint.\n",
			want:   []string{"(2 lines)"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			agenticRC := &rc.AgenticRC{Run: rc.RCRun{Instructions: rc.RCInstructions{Custom: tc.custom}}}

			// Act
			result := customInstructions(agenticRC)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func Test_showConfig(t *testing.T) {
	// Arrange
	withTempToolHome(t)
	dir := t.TempDir()
	writeRC(t, dir, "root = true\nnamespace = \"work\"\n")
	t.Chdir(dir)

	// Act
	var err error
	out := iostreamstest.CaptureOut(t, func() { err = showConfig(&cobra.Command{}, nil) })

	// Assert - the global section, then the project layers from the working dir
	require.NoError(t, err)
	assert.Contains(t, out, "Global (")
	assert.Contains(t, out, "Project (.agenticrc.toml, 1 file)")
	assert.Contains(t, out, "work")
	assert.Less(t, strings.Index(out, "Global ("), strings.Index(out, "Project ("))
}
