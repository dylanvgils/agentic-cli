package tools

import (
	"io/fs"
	"os/exec"
	"strings"
	"testing"

	df "github.com/dylanvgils/agentic-cli/internal/image/dockerfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_renderScript(t *testing.T) {
	names, err := fs.Glob(scriptFS, "scripts/*.sh")
	require.NoError(t, err)
	require.NotEmpty(t, names)

	for _, name := range names {
		t.Run("renders "+name, func(t *testing.T) {
			// Act
			lines := renderScript(strings.TrimPrefix(name, "scripts/"), defaultScriptVars())

			// Assert
			assert.NotEmpty(t, lines)
			assert.NotContains(t, strings.Join(lines, "\n"), "{{")
		})
	}

	t.Run("fills in the vars without a trailing empty line", func(t *testing.T) {
		// Arrange
		vars := defaultScriptVars()
		vars.MavenPortEnv = "TEST_PORT"
		vars.MavenSettingsTemplate = "/test/template.xml"

		// Act
		lines := renderScript("mavensettings.sh", vars)

		// Assert
		assert.Equal(t, `if [[ -n "${TEST_PORT:-}" && -f /test/template.xml ]]; then`, lines[1])
		assert.Equal(t, "fi", lines[len(lines)-1])
	})

	t.Run("panics on an unknown script", func(t *testing.T) {
		// Act + Assert
		assert.Panics(t, func() { renderScript("test-missing.sh", defaultScriptVars()) })
	})
}

func TestToolEntrypoints_shellcheck(t *testing.T) {
	if _, err := exec.LookPath("shellcheck"); err != nil {
		t.Skip("shellcheck not available")
	}
	scripts := map[string]df.Heredoc{
		"dockerd entrypoint": dockerdEntrypointScript(),
		"dockerd init":       dockerdInitScript(),
	}
	for _, img := range []ImageInfo{{}, {Extras: []string{"java"}}} {
		suffix := " " + strings.Join(img.Extras, ",")
		scripts["claude"+suffix] = claudeEntrypoint(img)
		scripts["copilot"+suffix] = copilotEntrypoint(img)
		scripts["opencode"+suffix] = opencodeEntrypoint(img)
	}

	for name, script := range scripts {
		t.Run(name+" passes shellcheck", func(t *testing.T) {
			// Arrange
			cmd := exec.Command("shellcheck", "-")
			cmd.Stdin = strings.NewReader(heredocBody(script))

			// Act
			out, err := cmd.CombinedOutput()

			// Assert
			assert.NoError(t, err, string(out))
		})
	}
}
