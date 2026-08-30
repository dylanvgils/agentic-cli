package docker

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildBaseArgs(t *testing.T) {
	t.Run("security flags", func(t *testing.T) {
		// Act
		args, err := buildBaseArgs(RunSpec{Image: "agentic-claude"})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, args, "run")
		assert.Contains(t, args, "--rm")
		assert.Contains(t, args, "--read-only")
		assert.Contains(t, args, "--cap-drop=ALL")
		assert.Contains(t, args, "--security-opt=no-new-privileges:true")
		assert.Contains(t, args, "--user="+platform.UserGroup())
	})

	t.Run("name and label", func(t *testing.T) {
		// Act
		args, err := buildBaseArgs(RunSpec{Image: "agentic-claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, hasArgWithPrefix(args, "--name=agentic-claude-"), "tool container should be named after its image")
		assert.Contains(t, args, "--label=project=agentic-cli")
	})

	t.Run("resource limits from spec", func(t *testing.T) {
		// Arrange
		rs := RunSpec{
			Image:     "agentic-claude",
			PidsLimit: "512",
			CPUs:      "2",
			Memory:    "2g",
		}

		// Act
		args, err := buildBaseArgs(rs)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, args, "--pids-limit=512")
		assert.Contains(t, args, "--cpus=2")
		assert.Contains(t, args, "--memory=2g")
	})

}

func TestBuildTTYArgs(t *testing.T) {
	t.Run("returns -it when terminal", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, true)

		// Act
		args := buildTTYArgs()

		// Assert
		assert.Equal(t, []string{"--interactive", "--tty"}, args)
	})

	t.Run("empty when not a terminal", func(t *testing.T) {
		// Arrange
		stubIsTerminal(t, false)

		// Act
		args := buildTTYArgs()

		// Assert
		assert.Empty(t, args)
	})
}

func TestBuildEnvArgs(t *testing.T) {
	clearTerminalEnv := func(t *testing.T) {
		for _, key := range terminalCapabilityEnvNames {
			t.Setenv(key, "")
		}
	}
	stubHostTimezone(t, "")

	t.Run("empty when no color vars set and no rs.Env", func(t *testing.T) {
		// Arrange
		clearTerminalEnv(t)

		// Act
		args := buildEnvArgs(RunSpec{})

		// Assert
		assert.Empty(t, args)
	})

	t.Run("COLORTERM passed through from host", func(t *testing.T) {
		// Arrange
		t.Setenv("COLORTERM", "truecolor")

		// Act
		args := buildEnvArgs(RunSpec{})

		// Assert
		assert.Contains(t, args, "--env=COLORTERM=truecolor")
	})

	t.Run("TERM passed through from host", func(t *testing.T) {
		// Arrange
		t.Setenv("TERM", "xterm-256color")

		// Act
		args := buildEnvArgs(RunSpec{})

		// Assert
		assert.Contains(t, args, "--env=TERM=xterm-256color")
	})

	t.Run("NO_COLOR passed through from host", func(t *testing.T) {
		// Arrange
		t.Setenv("NO_COLOR", "1")

		// Act
		args := buildEnvArgs(RunSpec{})

		// Assert
		assert.Contains(t, args, "--env=NO_COLOR=1")
	})

	t.Run("FORCE_COLOR passed through from host", func(t *testing.T) {
		// Arrange
		t.Setenv("FORCE_COLOR", "1")

		// Act
		args := buildEnvArgs(RunSpec{})

		// Assert
		assert.Contains(t, args, "--env=FORCE_COLOR=1")
	})

	t.Run("literal KEY=VALUE passed through", func(t *testing.T) {
		// Arrange
		clearTerminalEnv(t)
		rs := RunSpec{Env: []string{"MAVEN_OPTS=-Dfoo=bar"}}

		// Act
		args := buildEnvArgs(rs)

		// Assert
		assert.Equal(t, []string{"--env=MAVEN_OPTS=-Dfoo=bar"}, args)
	})

	t.Run("bare key forwards host value", func(t *testing.T) {
		// Arrange
		clearTerminalEnv(t)
		t.Setenv("CI", "true")
		rs := RunSpec{Env: []string{"CI"}}

		// Act
		args := buildEnvArgs(rs)

		// Assert
		assert.Equal(t, []string{"--env=CI=true"}, args)
	})

	t.Run("bare key omitted when unset on host", func(t *testing.T) {
		// Arrange
		clearTerminalEnv(t)
		os.Unsetenv("AGENTIC_NONEXISTENT_VAR") //nolint:errcheck
		rs := RunSpec{Env: []string{"AGENTIC_NONEXISTENT_VAR"}}

		// Act
		args := buildEnvArgs(rs)

		// Assert
		assert.Empty(t, args)
	})

	t.Run("user-supplied entry overrides auto-forwarded terminal var", func(t *testing.T) {
		// Arrange
		clearTerminalEnv(t)
		t.Setenv("NO_COLOR", "1")
		rs := RunSpec{Env: []string{"NO_COLOR=0"}}

		// Act
		args := buildEnvArgs(rs)

		// Assert - both occurrences are present; docker keeps the last one
		assert.Equal(t, []string{"--env=NO_COLOR=1", "--env=NO_COLOR=0"}, args)
	})

	t.Run("TZ forwarded when host timezone detected", func(t *testing.T) {
		// Arrange
		clearTerminalEnv(t)
		stubHostTimezone(t, "America/New_York")

		// Act
		args := buildEnvArgs(RunSpec{})

		// Assert
		assert.Equal(t, []string{"--env=TZ=America/New_York"}, args)
	})

	t.Run("no TZ arg when host timezone can't be detected", func(t *testing.T) {
		// Arrange
		clearTerminalEnv(t)
		stubHostTimezone(t, "")

		// Act
		args := buildEnvArgs(RunSpec{})

		// Assert
		assert.Empty(t, args)
	})

	t.Run("user-supplied entry overrides auto-forwarded TZ", func(t *testing.T) {
		// Arrange
		clearTerminalEnv(t)
		stubHostTimezone(t, "America/New_York")
		rs := RunSpec{Env: []string{"TZ=UTC"}}

		// Act
		args := buildEnvArgs(rs)

		// Assert - both occurrences are present; docker keeps the last one
		assert.Equal(t, []string{"--env=TZ=America/New_York", "--env=TZ=UTC"}, args)
	})
}

func TestBuildTmpfsArgs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		// Act
		args := buildTmpfsArgs(RunSpec{Image: "agentic-claude"})

		// Assert
		assert.Empty(t, args)
	})

	t.Run("expands container home", func(t *testing.T) {
		// Arrange
		rs := RunSpec{
			Image:         "agentic-copilot",
			ContainerHome: "/home/user",
			TmpfsMounts:   []string{"/tmp:exec,size=1g", "$CONTAINER_HOME/.cache:exec,size=1g"},
		}

		// Act
		args := buildTmpfsArgs(rs)

		// Assert
		assert.Equal(t, []string{
			"--tmpfs=/tmp:exec,size=1g",
			"--tmpfs=/home/user/.cache:exec,size=1g",
		}, args)
	})
}

func TestBuildVolumeArgs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		// Act
		args := buildVolumeArgs(RunSpec{Image: "agentic-claude"})

		// Assert
		assert.Empty(t, args)
	})

	t.Run("expands tool home", func(t *testing.T) {
		// Arrange
		rs := RunSpec{
			Image:    "agentic-claude",
			ToolHome: "/home/.agentic",
			Volumes:  []string{"/host:/container", "$TOOL_HOME/data:/data"},
		}

		// Act
		args := buildVolumeArgs(rs)

		// Assert
		assert.Equal(t, []string{
			"--volume=/host:/container",
			"--volume=/home/.agentic/data:/data",
		}, args)
	})
}

func TestBuildSecretArgs(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		// Arrange
		rs := RunSpec{
			Image:   "agentic-copilot",
			Secrets: []string{"mytoken:/tmp/token"},
		}

		// Act
		args, err := buildSecretArgs(rs)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--volume=/tmp/token:/run/secrets/mytoken:ro"}, args)
	})

	t.Run("invalid format", func(t *testing.T) {
		// Arrange
		rs := RunSpec{
			Image:   "agentic-copilot",
			Secrets: []string{"badformat"},
		}

		// Act
		_, err := buildSecretArgs(rs)

		// Assert
		assert.ErrorContains(t, err, "invalid secret")
	})

	t.Run("tilde expanded", func(t *testing.T) {
		// Arrange
		home, err := os.UserHomeDir()
		require.NoError(t, err)

		rs := RunSpec{
			Image:   "agentic-copilot",
			Secrets: []string{"mytoken:~/secrets/token"},
		}

		// Act
		args, err := buildSecretArgs(rs)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--volume=" + home + "/secrets/token:/run/secrets/mytoken:ro"}, args)
	})
}
