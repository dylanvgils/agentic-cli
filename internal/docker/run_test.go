package docker

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunContainer(t *testing.T) {
	get := stubRunInteractive(t)

	t.Run("security args", func(t *testing.T) {
		// Arrange
		rs := RunSpec{Image: "agentic-claude"}

		// Act
		err := RunContainer(rs, nil)

		// Assert
		require.NoError(t, err)
		args := get()
		assert.Contains(t, args, "run")
		assert.Contains(t, args, "--rm")
		assert.Contains(t, args, "--read-only")
		assert.Contains(t, args, "--cap-drop=ALL")
		assert.Contains(t, args, "--security-opt=no-new-privileges:true")
		assert.Contains(t, args, "--user="+platform.UserGroup())
	})

	t.Run("proxy mode swaps network and injects proxy env", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, "network inspect")
		rs := RunSpec{
			Image:        "agentic-claude",
			ProxyEnabled: true,
			ProxyImage:   "default-proxy",
			ProxyAllow:   []string{"api.anthropic.com"},
			ProxyLogDir:  t.TempDir(),
		}

		// Act
		err := RunContainer(rs, nil)

		// Assert
		require.NoError(t, err)
		args := get()
		assert.True(t, hasArgWithPrefix(args, "--network=agentic-proxy-"), "tool should attach to the internal proxy net")
		assert.False(t, hasArg(args, "--network="+NetworkName), "tool should not be on the egress net directly")
		assert.True(t, hasArgWithPrefix(args, "--env=HTTPS_PROXY=http://agentic-proxy-"), "tool should get HTTPS_PROXY")
	})

	t.Run("tmpfs mounts", func(t *testing.T) {
		// Arrange
		rs := RunSpec{
			Image:       "agentic-claude",
			TmpfsMounts: []string{"/tmp:exec,size=1g"},
		}

		// Act
		err := RunContainer(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, get(), "--tmpfs=/tmp:exec,size=1g")
	})

	t.Run("tmpfs mounts expand container home", func(t *testing.T) {
		// Arrange
		rs := RunSpec{
			Image:         "agentic-copilot",
			ContainerHome: "/home/user",
			TmpfsMounts:   []string{"/tmp:exec,size=1g", "$CONTAINER_HOME/.cache:exec,size=1g"},
		}

		// Act
		err := RunContainer(rs, nil)

		// Assert
		require.NoError(t, err)
		args := get()
		assert.Contains(t, args, "--tmpfs=/tmp:exec,size=1g")
		assert.Contains(t, args, "--tmpfs=/home/user/.cache:exec,size=1g")
	})

	t.Run("image and tool args", func(t *testing.T) {
		// Arrange
		rs := RunSpec{Image: "agentic-claude"}

		// Act
		err := RunContainer(rs, []string{"--resume"})

		// Assert
		require.NoError(t, err)
		args := get()
		n := len(args)
		require.GreaterOrEqual(t, n, 2)
		assert.Equal(t, "agentic-claude", args[n-2])
		assert.Equal(t, "--resume", args[n-1])
	})

	t.Run("skip entrypoint", func(t *testing.T) {
		// Arrange
		rs := RunSpec{
			Image:          "agentic-claude",
			SkipEntrypoint: true,
		}

		// Act
		err := RunContainer(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "", argAfter(get(), "--entrypoint"))
	})

	t.Run("volumes", func(t *testing.T) {
		// Arrange
		rs := RunSpec{
			Image:    "agentic-claude",
			ToolHome: "/home/.agentic",
			Volumes:  []string{"/host:/container", "$TOOL_HOME/data:/data"},
		}

		// Act
		err := RunContainer(rs, nil)

		// Assert
		require.NoError(t, err)
		args := get()
		assert.Contains(t, args, "--volume=/host:/container")
		assert.Contains(t, args, "--volume=/home/.agentic/data:/data")
	})

	t.Run("secrets", func(t *testing.T) {
		// Arrange
		rs := RunSpec{
			Image:   "agentic-copilot",
			Secrets: []string{"mytoken:/tmp/token"},
		}

		// Act
		err := RunContainer(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, get(), "--volume=/tmp/token:/run/secrets/mytoken:ro")
	})

	t.Run("secrets tilde expanded", func(t *testing.T) {
		// Arrange
		home, err := os.UserHomeDir()
		require.NoError(t, err)

		rs := RunSpec{
			Image:   "agentic-copilot",
			Secrets: []string{"mytoken:~/secrets/token"},
		}

		// Act
		err = RunContainer(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, get(), "--volume="+home+"/secrets/token:/run/secrets/mytoken:ro")
	})

	t.Run("secrets dollar HOME expanded", func(t *testing.T) {
		// Arrange
		home, err := os.UserHomeDir()
		require.NoError(t, err)

		rs := RunSpec{
			Image:   "agentic-copilot",
			Secrets: []string{"mytoken:$HOME/secrets/token"},
		}

		// Act
		err = RunContainer(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, get(), "--volume="+home+"/secrets/token:/run/secrets/mytoken:ro")
	})

	t.Run("secrets dollar HOME braces expanded", func(t *testing.T) {
		// Arrange
		home, err := os.UserHomeDir()
		require.NoError(t, err)

		rs := RunSpec{
			Image:   "agentic-copilot",
			Secrets: []string{"mytoken:${HOME}/secrets/token"},
		}

		// Act
		err = RunContainer(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, get(), "--volume="+home+"/secrets/token:/run/secrets/mytoken:ro")
	})

	t.Run("secrets with explicit container path", func(t *testing.T) {
		// Arrange
		rs := RunSpec{
			Image:   "agentic-copilot",
			Secrets: []string{"maven-settings:/tmp/settings.xml:/root/.m2/settings.xml"},
		}

		// Act
		err := RunContainer(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, get(), "--volume=/tmp/settings.xml:/root/.m2/settings.xml:ro")
	})

	t.Run("secrets with $CONTAINER_HOME in container path", func(t *testing.T) {
		// Arrange
		rs := RunSpec{
			Image:         "agentic-copilot",
			ContainerHome: "/root",
			Secrets:       []string{"maven-settings:/tmp/settings.xml:$CONTAINER_HOME/.m2/settings.xml"},
		}

		// Act
		err := RunContainer(rs, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, get(), "--volume=/tmp/settings.xml:/root/.m2/settings.xml:ro")
	})

	t.Run("secrets empty container path", func(t *testing.T) {
		// Arrange
		rs := RunSpec{
			Image:   "agentic-copilot",
			Secrets: []string{"mytoken:/tmp/token:"},
		}

		// Act + Assert
		assert.ErrorContains(t, RunContainer(rs, nil), "empty container path")
	})

	t.Run("secrets invalid format", func(t *testing.T) {
		// Arrange
		rs := RunSpec{
			Image:   "agentic-copilot",
			Secrets: []string{"badformat"},
		}

		// Act + Assert
		assert.ErrorContains(t, RunContainer(rs, nil), "invalid secret")
	})
}

func TestBuildBaseArgs(t *testing.T) {
	t.Run("security flags", func(t *testing.T) {
		// Act
		args := buildBaseArgs(RunSpec{Image: "agentic-claude"})

		// Assert
		assert.Contains(t, args, "run")
		assert.Contains(t, args, "--rm")
		assert.Contains(t, args, "--read-only")
		assert.Contains(t, args, "--cap-drop=ALL")
		assert.Contains(t, args, "--security-opt=no-new-privileges:true")
		assert.Contains(t, args, "--user="+platform.UserGroup())
	})

	t.Run("resource limits defaults", func(t *testing.T) {
		// Act
		args := buildBaseArgs(RunSpec{Image: "agentic-claude"})

		// Assert
		assert.Contains(t, args, "--pids-limit="+DefaultPidsLimit)
		assert.Contains(t, args, "--cpus="+DefaultCPUs)
		assert.Contains(t, args, "--memory="+DefaultMemory)
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
		args := buildBaseArgs(rs)

		// Assert
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
	t.Run("empty when no color vars set", func(t *testing.T) {
		// Arrange
		t.Setenv("COLORTERM", "")
		t.Setenv("TERM", "")
		t.Setenv("NO_COLOR", "")
		t.Setenv("FORCE_COLOR", "")

		// Act
		args := buildEnvArgs()

		// Assert
		assert.Empty(t, args)
	})

	t.Run("COLORTERM passed through from host", func(t *testing.T) {
		// Arrange
		t.Setenv("COLORTERM", "truecolor")

		// Act
		args := buildEnvArgs()

		// Assert
		assert.Contains(t, args, "--env=COLORTERM=truecolor")
	})

	t.Run("TERM passed through from host", func(t *testing.T) {
		// Arrange
		t.Setenv("TERM", "xterm-256color")

		// Act
		args := buildEnvArgs()

		// Assert
		assert.Contains(t, args, "--env=TERM=xterm-256color")
	})

	t.Run("NO_COLOR passed through from host", func(t *testing.T) {
		// Arrange
		t.Setenv("NO_COLOR", "1")

		// Act
		args := buildEnvArgs()

		// Assert
		assert.Contains(t, args, "--env=NO_COLOR=1")
	})

	t.Run("FORCE_COLOR passed through from host", func(t *testing.T) {
		// Arrange
		t.Setenv("FORCE_COLOR", "1")

		// Act
		args := buildEnvArgs()

		// Assert
		assert.Contains(t, args, "--env=FORCE_COLOR=1")
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
