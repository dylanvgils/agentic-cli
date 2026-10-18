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
		assert.Contains(t, result, "ARG DOCKER_VERSION="+DefaultVersions.Docker)
		assert.Contains(t, result, `"docker-ce=${CLI_VERSION}" "docker-ce-cli=${CLI_VERSION}" containerd.io`)
		assert.Contains(t, result, "iptables")
	})

	t.Run("gives the host gid the socket", func(t *testing.T) {
		// Act
		result := GenerateDockerdDockerfile("agentic-claude", "claude")

		// Assert
		assert.Contains(t, result, `printf '{"group": "%s"}\n' "${HOST_GID}" > /etc/docker/daemon.json`)
	})

	t.Run("init runs dockerd under tini without msb's args", func(t *testing.T) {
		// Act
		result := GenerateDockerdDockerfile("agentic-claude", "claude")

		// Assert
		assert.Contains(t, result, "COPY --chmod=0755 <<'EOF' "+DockerdInit)
		assert.Contains(t, result, "exec /usr/libexec/docker/docker-init -- dockerd\n")
	})

	t.Run("drops back to the tool user", func(t *testing.T) {
		// Act
		result := GenerateDockerdDockerfile("agentic-claude", "copilot")

		// Assert
		assert.Regexp(t, `USER copilot\s*$`, result)
	})
}
