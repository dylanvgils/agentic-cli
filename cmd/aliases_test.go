package cmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunAliases(t *testing.T) {
	t.Run("prints bash preamble and reload alias", func(t *testing.T) {
		// Arrange
		stubCheckDockerDaemon(t, func() error { return nil })
		stubBuiltTools(t, func() (map[string]bool, error) { return nil, nil })
		t.Setenv("SHELL", "/bin/bash")

		// Act
		out := captureStdout(t, func() {
			err := runAliases(aliasesCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "# agentic tool aliases - source with: source <(agentic aliases)")
		assert.Contains(t, out, "alias agentic-reload='source <(agentic aliases)'")
	})

	t.Run("prints fish preamble and reload function", func(t *testing.T) {
		// Arrange
		stubCheckDockerDaemon(t, func() error { return nil })
		stubBuiltTools(t, func() (map[string]bool, error) { return nil, nil })
		t.Setenv("SHELL", "/usr/bin/fish")

		// Act
		out := captureStdout(t, func() {
			err := runAliases(aliasesCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "# agentic tool aliases - source with: agentic aliases | source")
		assert.Contains(t, out, "function agentic-reload; agentic aliases | source; end")
	})

	t.Run("prints powershell preamble and reload function", func(t *testing.T) {
		// Arrange
		stubCheckDockerDaemon(t, func() error { return nil })
		stubBuiltTools(t, func() (map[string]bool, error) { return nil, nil })
		t.Setenv("SHELL", "/usr/bin/pwsh")

		// Act
		out := captureStdout(t, func() {
			err := runAliases(aliasesCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "# agentic tool aliases - source with: agentic aliases | Out-String | Invoke-Expression")
		assert.Contains(t, out, "function agentic-reload { agentic aliases | Out-String | Invoke-Expression }")
	})

	t.Run("prints powershell preamble and reload function on windows", func(t *testing.T) {
		// Arrange
		stubCheckDockerDaemon(t, func() error { return nil })
		stubBuiltTools(t, func() (map[string]bool, error) { return nil, nil })
		stubCurrentGOOS(t, "windows")
		t.Setenv("SHELL", "")

		// Act
		out := captureStdout(t, func() {
			err := runAliases(aliasesCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "# agentic tool aliases - source with: agentic aliases | Out-String | Invoke-Expression")
		assert.Contains(t, out, "function agentic-reload { agentic aliases | Out-String | Invoke-Expression }")
	})

	t.Run("not built tools emit no tool aliases", func(t *testing.T) {
		// Arrange
		stubCheckDockerDaemon(t, func() error { return nil })
		stubBuiltTools(t, func() (map[string]bool, error) { return nil, nil })
		t.Setenv("SHELL", "/bin/bash")

		// Act
		out := captureStdout(t, func() {
			err := runAliases(aliasesCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.NotContains(t, out, "alias claude=")
		assert.NotContains(t, out, "alias copilot=")
		assert.NotContains(t, out, "alias opencode=")
	})

	t.Run("only built tools get aliases", func(t *testing.T) {
		// Arrange - only claude is built
		stubCheckDockerDaemon(t, func() error { return nil })
		stubBuiltTools(t, func() (map[string]bool, error) {
			return map[string]bool{"claude": true}, nil
		})
		t.Setenv("SHELL", "/bin/bash")

		// Act
		out := captureStdout(t, func() {
			err := runAliases(aliasesCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "alias claude='agentic run claude'")
		assert.NotContains(t, out, "alias copilot=")
		assert.NotContains(t, out, "alias opencode=")
	})

	t.Run("built tools emit bash alias lines", func(t *testing.T) {
		// Arrange
		stubCheckDockerDaemon(t, func() error { return nil })
		stubBuiltTools(t, func() (map[string]bool, error) {
			return map[string]bool{"claude": true, "copilot": true, "opencode": true}, nil
		})
		t.Setenv("SHELL", "/bin/bash")

		// Act
		out := captureStdout(t, func() {
			err := runAliases(aliasesCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "alias claude='agentic run claude'")
		assert.Contains(t, out, "alias copilot='agentic run copilot'")
		assert.Contains(t, out, "alias opencode='agentic run opencode'")
	})

	t.Run("built tools emit powershell function lines", func(t *testing.T) {
		// Arrange
		stubCheckDockerDaemon(t, func() error { return nil })
		stubBuiltTools(t, func() (map[string]bool, error) {
			return map[string]bool{"claude": true, "copilot": true, "opencode": true}, nil
		})
		t.Setenv("SHELL", "/usr/bin/pwsh")

		// Act
		out := captureStdout(t, func() {
			err := runAliases(aliasesCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "function claude { agentic run claude @args }")
		assert.Contains(t, out, "function copilot { agentic run copilot @args }")
		assert.Contains(t, out, "function opencode { agentic run opencode @args }")
	})

	t.Run("daemon unreachable prints preamble and reload but no tool aliases", func(t *testing.T) {
		// Arrange
		stubCheckDockerDaemon(t, func() error { return fmt.Errorf("docker daemon not running") })
		t.Setenv("SHELL", "/bin/bash")

		// Act
		out := captureStdout(t, func() {
			err := runAliases(aliasesCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "# agentic tool aliases")
		assert.Contains(t, out, "alias agentic-reload=")
		assert.NotContains(t, out, "alias claude=")
		assert.NotContains(t, out, "alias copilot=")
		assert.NotContains(t, out, "alias opencode=")
	})

	t.Run("built tools error prints preamble and reload but no tool aliases", func(t *testing.T) {
		// Arrange
		stubCheckDockerDaemon(t, func() error { return nil })
		stubBuiltTools(t, func() (map[string]bool, error) {
			return nil, fmt.Errorf("image list failed")
		})
		t.Setenv("SHELL", "/bin/bash")

		// Act
		out := captureStdout(t, func() {
			err := runAliases(aliasesCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "# agentic tool aliases")
		assert.Contains(t, out, "alias agentic-reload=")
		assert.NotContains(t, out, "alias claude=")
		assert.NotContains(t, out, "alias copilot=")
		assert.NotContains(t, out, "alias opencode=")
	})
}

func Test_printAliases(t *testing.T) {
	t.Run("no built tools prints nothing", func(t *testing.T) {
		// Act
		out := captureStdout(t, func() {
			printAliases("bash", map[string]bool{})
		})

		// Assert
		assert.Empty(t, out)
	})

	t.Run("only built tools appear in output", func(t *testing.T) {
		// Arrange
		built := map[string]bool{"claude": true}

		// Act
		out := captureStdout(t, func() {
			printAliases("bash", built)
		})

		// Assert
		assert.Contains(t, out, "alias claude=")
		assert.NotContains(t, out, "alias copilot=")
		assert.NotContains(t, out, "alias opencode=")
	})
}

func Test_shellFromEnv(t *testing.T) {
	t.Run("bash", func(t *testing.T) {
		// Arrange
		t.Setenv("SHELL", "/bin/bash")

		// Act
		result := shellFromEnv()

		// Assert
		assert.Equal(t, "bash", result)
	})

	t.Run("sh", func(t *testing.T) {
		// Arrange
		t.Setenv("SHELL", "/bin/sh")

		// Act
		result := shellFromEnv()

		// Assert
		assert.Equal(t, "bash", result)
	})

	t.Run("zsh", func(t *testing.T) {
		// Arrange
		t.Setenv("SHELL", "/bin/zsh")

		// Act
		result := shellFromEnv()

		// Assert
		assert.Equal(t, "zsh", result)
	})

	t.Run("fish", func(t *testing.T) {
		// Arrange
		t.Setenv("SHELL", "/usr/bin/fish")

		// Act
		result := shellFromEnv()

		// Assert
		assert.Equal(t, "fish", result)
	})

	t.Run("pwsh", func(t *testing.T) {
		// Arrange
		t.Setenv("SHELL", "/usr/bin/pwsh")

		// Act
		result := shellFromEnv()

		// Assert
		assert.Equal(t, "powershell", result)
	})

	t.Run("unknown returns empty", func(t *testing.T) {
		// Arrange
		t.Setenv("SHELL", "/usr/bin/dash")

		// Act
		result := shellFromEnv()

		// Assert
		assert.Equal(t, "", result)
	})

	t.Run("unset returns empty", func(t *testing.T) {
		// Arrange
		t.Setenv("SHELL", "")

		// Act
		result := shellFromEnv()

		// Assert
		assert.Equal(t, "", result)
	})
}

func Test_defaultShell(t *testing.T) {
	t.Run("windows returns powershell", func(t *testing.T) {
		// Arrange
		stubCurrentGOOS(t, "windows")

		// Act
		result := defaultShell()

		// Assert
		assert.Equal(t, "powershell", result)
	})

	t.Run("non-windows returns bash", func(t *testing.T) {
		// Arrange
		stubCurrentGOOS(t, "linux")

		// Act
		result := defaultShell()

		// Assert
		assert.Equal(t, "bash", result)
	})
}

func Test_detectShell(t *testing.T) {
	t.Run("prefers SHELL over OS default", func(t *testing.T) {
		// Arrange
		stubCurrentGOOS(t, "windows")
		t.Setenv("SHELL", "/usr/bin/bash")

		// Act
		result := detectShell()

		// Assert
		assert.Equal(t, "bash", result)
	})

	t.Run("falls back to OS default when SHELL unset", func(t *testing.T) {
		// Arrange
		stubCurrentGOOS(t, "windows")
		t.Setenv("SHELL", "")

		// Act
		result := detectShell()

		// Assert
		assert.Equal(t, "powershell", result)
	})
}

func Test_reloadLineFor(t *testing.T) {
	t.Run("bash returns alias", func(t *testing.T) {
		// Act
		result := reloadLineFor("bash")

		// Assert
		assert.Equal(t, "alias agentic-reload='source <(agentic aliases)'", result)
	})

	t.Run("zsh returns alias", func(t *testing.T) {
		// Act
		result := reloadLineFor("zsh")

		// Assert
		assert.Equal(t, "alias agentic-reload='source <(agentic aliases)'", result)
	})

	t.Run("fish returns function", func(t *testing.T) {
		// Act
		result := reloadLineFor("fish")

		// Assert
		assert.Equal(t, "function agentic-reload; agentic aliases | source; end", result)
	})

	t.Run("powershell returns function", func(t *testing.T) {
		// Act
		result := reloadLineFor("powershell")

		// Assert
		assert.Equal(t, "function agentic-reload { agentic aliases | Out-String | Invoke-Expression }", result)
	})
}
