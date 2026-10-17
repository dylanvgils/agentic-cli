package docker

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRunArgs(t *testing.T) {
	// Act
	_, err := buildRunArgs(toolRun{RunSpec: sandbox.RunSpec{Image: "agentic-claude"}}, nil, nil)

	// Assert
	assert.ErrorContains(t, err, "no host user", "without --user the tool would run as the image's user")
}

func TestBuildBaseArgs(t *testing.T) {
	t.Run("security flags", func(t *testing.T) {
		// Act
		args := buildBaseArgs(toolRun{RunSpec: sandbox.RunSpec{Image: "agentic-claude", User: testUser}})

		// Assert
		assert.Contains(t, args, "run")
		assert.Contains(t, args, "--rm")
		assert.Contains(t, args, "--read-only")
		assert.Contains(t, args, "--cap-drop=ALL")
		assert.Contains(t, args, "--security-opt=no-new-privileges:true")
		assert.Contains(t, args, "--user="+testUser)
	})

	t.Run("name and label", func(t *testing.T) {
		// Act
		args := buildBaseArgs(toolRun{RunSpec: sandbox.RunSpec{Image: "agentic-claude"}, container: "agentic-claude-0123456789ab"})

		// Assert
		assert.Contains(t, args, "--name=agentic-claude-0123456789ab")
		assert.Contains(t, args, "--label=project=agentic-cli")
	})

	t.Run("resource limits from spec", func(t *testing.T) {
		// Arrange
		rs := toolRun{RunSpec: sandbox.RunSpec{
			Image:  "agentic-claude",
			Limits: sandbox.ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"},
		}}

		// Act
		args := buildBaseArgs(rs)

		// Assert
		assert.Contains(t, args, "--pids-limit=512")
		assert.Contains(t, args, "--cpus=2")
		assert.Contains(t, args, "--memory=2g")
	})

}

func TestBuildTTYArgs(t *testing.T) {
	t.Run("returns -it when terminal", func(t *testing.T) {
		// Act
		args := buildTTYArgs(sandbox.RunSpec{TTY: true})

		// Assert
		assert.Equal(t, []string{"--interactive", "--tty"}, args)
	})

	t.Run("empty when not a terminal", func(t *testing.T) {
		// Act
		args := buildTTYArgs(sandbox.RunSpec{})

		// Assert
		assert.Empty(t, args)
	})
}

func TestBuildTmpfsArgs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		// Act
		args := buildTmpfsArgs(sandbox.RunSpec{Image: "agentic-claude"})

		// Assert
		assert.Empty(t, args)
	})

	t.Run("expands container home", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{
			Image:         "agentic-copilot",
			ContainerHome: "/home/user",
			EphemeralDirs: []sandbox.EphemeralDir{{Path: "/tmp", Size: "1g"}, {Path: "$CONTAINER_HOME/.cache", Size: "1g"}},
		}

		// Act
		args := buildTmpfsArgs(rs)

		// Assert
		assert.Equal(t, []string{
			"--tmpfs=/tmp:exec,size=1g",
			"--tmpfs=/home/user/.cache:exec,size=1g",
		}, args)
	})
	t.Run("keeps every tool's tmpfs flags", func(t *testing.T) {
		want := map[string][]string{
			"claude":   {"--tmpfs=/tmp:exec,size=1g"},
			"copilot":  {"--tmpfs=/tmp:exec,size=1g", "--tmpfs=/home/user/.cache:exec,size=1g"},
			"opencode": {"--tmpfs=/tmp:exec,size=1g"},
		}
		require.Len(t, tools.Configs, len(want))

		for name, tool := range tools.Configs {
			// Arrange
			rs := sandbox.RunSpec{ContainerHome: "/home/user", EphemeralDirs: tool.Runtime.EphemeralDirs()}

			// Act
			args := buildTmpfsArgs(rs)

			// Assert
			assert.Equal(t, want[name], args, name)
		}
	})
}

func TestBuildVolumeArgs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		// Act
		args := buildVolumeArgs(sandbox.RunSpec{Image: "agentic-claude"})

		// Assert
		assert.Empty(t, args)
	})

	t.Run("expands tool home", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{
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
		rs := sandbox.RunSpec{
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
		rs := sandbox.RunSpec{
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

		rs := sandbox.RunSpec{
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
