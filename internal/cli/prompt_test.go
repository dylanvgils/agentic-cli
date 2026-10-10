package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ttyPrompter_TrustDir(t *testing.T) {
	t.Run("no tty returns hint to pass --trust-dir", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, false)

		// Act
		err := ttyPrompter{}.TrustDir("/example.test/project")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--trust-dir")
	})

	t.Run("tty answers y approves", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)
		stubStdin(t, "y\n")
		logs := stubErrLog(t)

		// Act
		err := ttyPrompter{}.TrustDir("/example.test/project")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logs.String(), "trust directory /example.test/project? [y/N]")
	})

	t.Run("tty answers n refuses", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)
		stubStdin(t, "n\n")
		stubErrLog(t)

		// Act
		err := ttyPrompter{}.TrustDir("/example.test/project")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not trusted")
	})
}

func Test_describeDir(t *testing.T) {
	t.Run("plain dir is shown as is", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()

		// Act
		got := describeDir(dir)

		// Assert
		assert.Equal(t, dir, got)
	})

	t.Run("symlinked dir shows where it leads", func(t *testing.T) {
		// Arrange
		if runtime.GOOS == "windows" {
			t.Skip("symlinks need privileges on windows")
		}
		resolved, err := filepath.EvalSymlinks(t.TempDir())
		require.NoError(t, err)
		link := filepath.Join(t.TempDir(), "sub")
		require.NoError(t, os.Symlink(resolved, link))

		// Act
		got := describeDir(link)

		// Assert
		assert.Equal(t, link+" (-> "+resolved+")", got)
	})
}

func Test_ttyPrompter_ApproveSettings(t *testing.T) {
	layer := config.RCLayer{Path: "/example.test/.agenticrc.toml", RC: &config.AgenticRC{}}
	changed := []config.GuardedSetting{{Key: "run.extra_mounts", Value: []string{"~/.example:/x:rw"}}}

	t.Run("no tty names the changed keys and hints to run interactively", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, false)

		// Act
		err := ttyPrompter{}.ApproveSettings(layer, changed)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "(run.extra_mounts); run interactively")
	})

	t.Run("tty answers y lists the settings and approves", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)
		stubStdin(t, "y\n")
		logs := stubErrLog(t)

		// Act
		err := ttyPrompter{}.ApproveSettings(layer, changed)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logs.String(), "approve only changes you made or expect")
		assert.Contains(t, logs.String(), "=> run.extra_mounts - host paths mounted into the container\n   \"~/.example:/x:rw\"\n")
	})

	t.Run("tty answers n refuses", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)
		stubStdin(t, "n\n")
		stubErrLog(t)

		// Act
		err := ttyPrompter{}.ApproveSettings(layer, changed)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not approved")
	})
}

func Test_ttyPrompter_OfferToolUpdate(t *testing.T) {
	t.Run("no tty prints one-liner and declines", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, false)
		logs := stubErrLog(t)

		// Act
		result := ttyPrompter{}.OfferToolUpdate("claude", "1.2.3", "1.3.0")

		// Assert
		assert.False(t, result)
		assert.Contains(t, logs.String(), "agentic: claude update available: 1.3.0 (current: 1.2.3) - run: agentic update claude")
	})

	t.Run("tty answers y accepts", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)
		stubStdin(t, "y\n")
		logs := stubErrLog(t)

		// Act
		result := ttyPrompter{}.OfferToolUpdate("claude", "1.2.3", "1.3.0")

		// Assert
		assert.True(t, result)
		assert.Contains(t, logs.String(), "claude update available: 1.3.0 (current: 1.2.3) - update now? [y/N]")
	})

	t.Run("tty answers n declines", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)
		stubStdin(t, "n\n")
		stubErrLog(t)

		// Act
		result := ttyPrompter{}.OfferToolUpdate("claude", "1.2.3", "1.3.0")

		// Assert
		assert.False(t, result)
	})
}

func Test_ttyPrompter_OfferUpgrade(t *testing.T) {
	t.Run("no tty prints one-liner and declines", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, false)
		logs := stubErrLog(t)

		// Act
		result := ttyPrompter{}.OfferUpgrade("v1.0.0", "v1.1.0")

		// Assert
		assert.False(t, result)
		assert.Equal(t, "=> agentic update available: v1.1.0 (current: v1.0.0) - run: agentic upgrade\n", logs.String())
	})

	t.Run("tty answers y accepts", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)
		stubStdin(t, "y\n")
		logs := stubErrLog(t)

		// Act
		result := ttyPrompter{}.OfferUpgrade("v1.0.0", "v1.1.0")

		// Assert
		assert.True(t, result)
		assert.Equal(t, "=> agentic update available: v1.1.0 (current: v1.0.0)\n   update now? [y/N] ", logs.String())
	})

	t.Run("tty answers n declines", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)
		stubStdin(t, "n\n")
		stubErrLog(t)

		// Act
		result := ttyPrompter{}.OfferUpgrade("v1.0.0", "v1.1.0")

		// Assert
		assert.False(t, result)
	})
}

func Test_describeCredential(t *testing.T) {
	t.Run("preset", func(t *testing.T) {
		// Arrange
		cred := config.RCCredential{Preset: "github", Secret: "~/.secrets/gh"}

		// Act
		target, details := describeCredential(cred)

		// Assert
		assert.Equal(t, `preset "github"`, target)
		assert.Equal(t, []string{`secret "~/.secrets/gh"`}, details)
	})

	t.Run("custom hosts with env", func(t *testing.T) {
		// Arrange
		cred := config.RCCredential{
			Hosts:  []string{"api.example.test", "*.example.test"},
			Header: "X-Token",
			Env:    []string{"EXAMPLE_TOKEN"},
			Secret: "/example.test/token",
		}

		// Act
		target, details := describeCredential(cred)

		// Assert
		assert.Equal(t, `header "X-Token" on "api.example.test", "*.example.test"`, target)
		assert.Equal(t, []string{`secret "/example.test/token"`, `env "EXAMPLE_TOKEN"`}, details)
	})

	t.Run("escape sequences print as text", func(t *testing.T) {
		// Arrange
		cred := config.RCCredential{
			Hosts:  []string{"evil.example.test\x1b[2K\r"},
			Header: "X-Token",
			Secret: "/example.test/key\u202e",
		}

		// Act
		target, details := describeCredential(cred)

		// Assert
		assert.Equal(t, `header "X-Token" on "evil.example.test\x1b[2K\r"`, target)
		assert.Equal(t, []string{`secret "/example.test/key\u202e"`}, details)
	})
}

func Test_describeSetting(t *testing.T) {
	t.Run("scalar value is shown as json under its key and effect", func(t *testing.T) {
		// Arrange
		disabled := false

		// Act
		header, values := describeSetting(config.GuardedSetting{Key: "run.proxy.enabled", Value: &disabled})

		// Assert
		assert.Equal(t, "run.proxy.enabled - "+settingEffects["run.proxy.enabled"], header)
		assert.Equal(t, []string{"false"}, values)
	})

	t.Run("list value is shown one item per line", func(t *testing.T) {
		// Act
		_, values := describeSetting(config.GuardedSetting{Key: "run.env", Value: []string{"A", "B"}})

		// Assert
		assert.Equal(t, []string{`"A"`, `"B"`}, values)
	})

	t.Run("cleared value is shown as removed", func(t *testing.T) {
		// Act
		header, values := describeSetting(config.GuardedSetting{Key: "run.read_only_mounts", Value: []string(nil)})

		// Assert
		assert.Equal(t, "run.read_only_mounts - "+settingEffects["run.read_only_mounts"]+" (removed)", header)
		assert.Empty(t, values)
	})
}

func Test_describeValue(t *testing.T) {
	t.Run("custom install shows its name and one command per line", func(t *testing.T) {
		// Arrange
		install := config.RCCustomInstall{Name: "example", Run: []string{"curl -fsSL https://example.test/install.sh | sh", "example --version"}}

		// Act
		lines := describeValue(install)

		// Assert
		assert.Equal(t, []string{`"example":`, `  "curl -fsSL https://example.test/install.sh | sh"`, `  "example --version"`}, lines)
	})

	t.Run("custom install command escape sequences print as text", func(t *testing.T) {
		// Arrange
		install := config.RCCustomInstall{Name: "example", Run: []string{"true\x1b[2K\r"}}

		// Act
		lines := describeValue(install)

		// Assert
		assert.Equal(t, `  "true\x1b[2K\r"`, lines[1])
	})

	t.Run("credential shows where it is sent with its details below", func(t *testing.T) {
		// Arrange
		cred := config.RCCredential{Preset: "github", Secret: "~/.secrets/gh"}

		// Act
		lines := describeValue(cred)

		// Assert
		assert.Equal(t, []string{`preset "github"`, `  secret "~/.secrets/gh"`}, lines)
	})

	t.Run("other value is shown as json", func(t *testing.T) {
		// Act
		lines := describeValue("~/.example:/x:rw")

		// Assert
		assert.Equal(t, []string{`"~/.example:/x:rw"`}, lines)
	})
}

func Test_settingEffects(t *testing.T) {
	// Arrange
	settings := config.GuardedSettings(&config.AgenticRC{})

	// Act
	var missing []string
	for _, setting := range settings {
		if settingEffects[setting.Key] == "" {
			missing = append(missing, setting.Key)
		}
	}

	// Assert
	assert.Empty(t, missing)
}

func Test_settingKeys(t *testing.T) {
	// Arrange
	settings := []config.GuardedSetting{{Key: "root"}, {Key: "run.env"}}

	// Act
	keys := settingKeys(settings)

	// Assert
	assert.Equal(t, "root, run.env", keys)
}

func Test_confirmed(t *testing.T) {
	t.Run("leaves the next line for the next prompt", func(t *testing.T) {
		// Arrange
		stubStdin(t, "y\nn\n")

		// Act
		first := confirmed()
		second := confirmed()

		// Assert
		assert.True(t, first)
		assert.False(t, second)
	})

	t.Run("accepts uppercase and surrounding spaces", func(t *testing.T) {
		// Arrange
		stubStdin(t, "  Y \n")

		// Act
		result := confirmed()

		// Assert
		assert.True(t, result)
	})

	t.Run("eof without newline", func(t *testing.T) {
		// Arrange
		stubStdin(t, "y")

		// Act
		result := confirmed()

		// Assert
		assert.True(t, result)
	})

	t.Run("empty input", func(t *testing.T) {
		// Arrange
		stubStdin(t, "")

		// Act
		result := confirmed()

		// Assert
		assert.False(t, result)
	})
}
