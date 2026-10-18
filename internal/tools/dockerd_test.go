package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerdImage(t *testing.T) {
	// Act
	result := DockerdImage("agentic-claude")

	// Assert
	assert.Equal(t, "agentic-claude-dockerd", result)
}

func TestGenerateDockerdDockerfile(t *testing.T) {
	t.Run("builds on the parent as root", func(t *testing.T) {
		// Act
		result := GenerateDockerdDockerfile("agentic-claude", "claude")

		// Assert
		assert.Contains(t, result, "FROM agentic-claude AS dockerd")
		assert.Contains(t, result, "USER root")
	})

	t.Run("installs the daemon pinned to the cli release", func(t *testing.T) {
		// Act
		result := GenerateDockerdDockerfile("agentic-claude", "claude")

		// Assert
		assert.Contains(t, result, `"docker-ce=$(dpkg-query --show --showformat='${Version}' docker-ce-cli)"`)
		assert.Contains(t, result, "containerd.io")
		assert.Contains(t, result, "iptables")
		assert.Contains(t, result, "busybox")
		assert.NotContains(t, result, "docker-ce-cli=")
	})

	t.Run("gives the tool user's primary group the socket", func(t *testing.T) {
		// Act
		result := GenerateDockerdDockerfile("agentic-claude", "claude")

		// Assert
		assert.Contains(t, result, `printf '{"group": "%s"}\n' "$(id -g claude)" > /etc/docker/daemon.json`)
		assert.NotContains(t, result, "HOST_GID")
	})

	t.Run("init runs dockerd without msb's args", func(t *testing.T) {
		// Act
		result := GenerateDockerdDockerfile("agentic-claude", "claude")

		// Assert
		assert.Contains(t, result, "COPY --chmod=0755 <<'EOF' "+DockerdInit)
		assert.Contains(t, result, "dockerd &\n")
	})

	t.Run("init stops dockerd on any shutdown signal", func(t *testing.T) {
		// Act
		result := GenerateDockerdDockerfile("agentic-claude", "claude")

		// Assert
		assert.Contains(t, result, "for sig in TERM INT PWR $(seq 34 64); do\n")
		assert.Contains(t, result, `kill -TERM "$pid"`)
	})

	t.Run("init powers the vm off once dockerd exits", func(t *testing.T) {
		// Act
		result := GenerateDockerdDockerfile("agentic-claude", "claude")

		// Assert
		assert.Contains(t, result, "exec busybox poweroff -f\n")
	})

	t.Run("drops back to the tool user", func(t *testing.T) {
		// Act
		result := GenerateDockerdDockerfile("agentic-claude", "copilot")

		// Assert
		assert.Regexp(t, `USER copilot\s*$`, result)
	})
}
