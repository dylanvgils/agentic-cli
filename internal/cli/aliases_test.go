package cli

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runAliases(t *testing.T) {
	t.Run("prints the shell's preamble, reload and short alias, then aliases for built tools only", func(t *testing.T) {
		// Arrange - only claude is built
		stubCheckDockerDaemon(t, func() error { return nil })
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{{Tool: "claude"}}, nil
		})
		t.Setenv("SHELL", "/usr/bin/fish")

		// Act
		var err error
		out := iostreamstest.CaptureOut(t, func() { err = runAliases(aliasesCmd, []string{}) })

		// Assert
		require.NoError(t, err)
		assert.Equal(t, preambleFor("fish")+"\n"+
			reloadLineFor("fish")+"\n"+
			shortAliasFor("fish")+"\n"+
			"alias claude='agentic run claude'\n", out)
	})

	for _, tc := range []struct {
		name      string
		daemonErr error
		listErr   error
	}{
		{name: "daemon unreachable", daemonErr: fmt.Errorf("docker daemon not running")},
		{name: "image list error", listErr: fmt.Errorf("image list failed")},
	} {
		t.Run(tc.name+" prints no tool aliases", func(t *testing.T) {
			// Arrange
			stubCheckDockerDaemon(t, func() error { return tc.daemonErr })
			stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
				return []*docker.ImageInfo{{Tool: "claude"}}, tc.listErr
			})
			t.Setenv("SHELL", "/bin/bash")

			// Act
			var err error
			out := iostreamstest.CaptureOut(t, func() { err = runAliases(aliasesCmd, []string{}) })

			// Assert
			require.NoError(t, err)
			assert.Equal(t, preambleFor("bash")+"\n"+reloadLineFor("bash")+"\n"+shortAliasFor("bash")+"\n", out)
		})
	}
}

func Test_printAliases(t *testing.T) {
	t.Run("no built tools prints nothing", func(t *testing.T) {
		// Act
		out := iostreamstest.CaptureOut(t, func() { printAliases("bash", map[string]bool{}) })

		// Assert
		assert.Empty(t, out)
	})

	t.Run("built tools print in name order", func(t *testing.T) {
		// Arrange
		built := map[string]bool{"opencode": true, "claude": true}

		// Act
		out := iostreamstest.CaptureOut(t, func() { printAliases("powershell", built) })

		// Assert
		assert.Equal(t, "function claude { agentic run claude @args }\nfunction opencode { agentic run opencode @args }\n", out)
	})
}

func Test_shellFromEnv(t *testing.T) {
	for _, tc := range []struct {
		shell string
		want  string
	}{
		{shell: "/bin/bash", want: "bash"},
		{shell: "/bin/sh", want: "bash"},
		{shell: "/bin/zsh", want: "zsh"},
		{shell: "/usr/bin/fish", want: "fish"},
		{shell: "/usr/bin/pwsh", want: "powershell"},
		{shell: "/usr/bin/dash", want: ""},
	} {
		t.Run(tc.shell, func(t *testing.T) {
			// Arrange
			t.Setenv("SHELL", tc.shell)

			// Act
			result := shellFromEnv()

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func Test_defaultShell(t *testing.T) {
	for goos, want := range map[string]string{
		"windows": "powershell",
		"linux":   "bash",
	} {
		t.Run(goos, func(t *testing.T) {
			// Act
			result := defaultShell(goos)

			// Assert
			assert.Equal(t, want, result)
		})
	}
}

func Test_detectShell(t *testing.T) {
	t.Run("prefers SHELL over OS default", func(t *testing.T) {
		// Arrange
		t.Setenv("SHELL", "/usr/bin/bash")

		// Act
		result := detectShell("windows")

		// Assert
		assert.Equal(t, "bash", result)
	})

	t.Run("falls back to OS default when SHELL unset", func(t *testing.T) {
		// Arrange
		t.Setenv("SHELL", "")

		// Act
		result := detectShell("windows")

		// Assert
		assert.Equal(t, "powershell", result)
	})
}

func Test_preambleFor(t *testing.T) {
	for shell, want := range map[string]string{
		"bash":       "# agentic tool aliases - source with: source <(agentic aliases)",
		"fish":       "# agentic tool aliases - source with: agentic aliases | source",
		"powershell": "# agentic tool aliases - source with: agentic aliases | Out-String | Invoke-Expression",
	} {
		t.Run(shell, func(t *testing.T) {
			// Act
			result := preambleFor(shell)

			// Assert
			assert.Equal(t, want, result)
		})
	}
}

func Test_aliasLineFor(t *testing.T) {
	for shell, want := range map[string]string{
		"bash":       "alias claude='agentic run claude'",
		"powershell": "function claude { agentic run claude @args }",
	} {
		t.Run(shell, func(t *testing.T) {
			// Act
			result := aliasLineFor(shell, "claude")

			// Assert
			assert.Equal(t, want, result)
		})
	}
}

func Test_reloadLineFor(t *testing.T) {
	for shell, want := range map[string]string{
		"bash":       "alias agentic-reload='source <(agentic aliases)'",
		"fish":       "function agentic-reload; agentic aliases | source; end",
		"powershell": "function agentic-reload { agentic aliases | Out-String | Invoke-Expression }",
	} {
		t.Run(shell, func(t *testing.T) {
			// Act
			result := reloadLineFor(shell)

			// Assert
			assert.Equal(t, want, result)
		})
	}
}

func Test_shortAliasFor(t *testing.T) {
	for shell, want := range map[string]string{
		"bash": "alias agt=agentic\n" + `c=$(complete -p agentic 2>/dev/null) && eval "${c% agentic} agt"`,
		"zsh":  "alias agt=agentic",
		"fish": "function agt --wraps agentic; agentic $argv; end",
		"powershell": "Set-Alias agt agentic\n" +
			"if ($__agenticCompleterBlock) { Register-ArgumentCompleter -Native -CommandName agt -ScriptBlock $__agenticCompleterBlock }",
	} {
		t.Run(shell, func(t *testing.T) {
			// Act
			result := shortAliasFor(shell)

			// Assert
			assert.Equal(t, want, result)
		})
	}
}
