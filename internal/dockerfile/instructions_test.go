package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFrom_render(t *testing.T) {
	t.Run("no As", func(t *testing.T) {
		// Act
		result := From{Image: "debian:bookworm-slim"}.Render()

		// Assert
		assert.Equal(t, "FROM debian:bookworm-slim", result)
	})

	t.Run("with As", func(t *testing.T) {
		// Act
		result := From{Image: "debian:bookworm-slim", As: "base"}.Render()

		// Assert
		assert.Equal(t, "FROM debian:bookworm-slim AS base", result)
	})
}

func TestArg_render(t *testing.T) {
	t.Run("no default", func(t *testing.T) {
		// Act
		result := Arg{Key: "HOST_UID"}.Render()

		// Assert
		assert.Equal(t, "ARG HOST_UID", result)
	})

	t.Run("with default", func(t *testing.T) {
		// Act
		result := Arg{Key: "NODE_VERSION", Default: "24"}.Render()

		// Assert
		assert.Equal(t, "ARG NODE_VERSION=24", result)
	})
}

func TestEnv_render(t *testing.T) {
	// Act
	result := Env{Key: "DEBIAN_FRONTEND", Value: "noninteractive"}.Render()

	// Assert
	assert.Equal(t, "ENV DEBIAN_FRONTEND=noninteractive", result)
}

func TestShell_render(t *testing.T) {
	// Act
	result := Shell{Cmd: []string{"/bin/bash", "-o", "pipefail", "-c"}}.Render()

	// Assert
	assert.Equal(t, `SHELL ["/bin/bash", "-o", "pipefail", "-c"]`, result)
}

func TestUser_render(t *testing.T) {
	// Act
	result := User{Name: "claude"}.Render()

	// Assert
	assert.Equal(t, "USER claude", result)
}

func TestWorkdir_render(t *testing.T) {
	// Act
	result := Workdir{Path: "/workspace"}.Render()

	// Assert
	assert.Equal(t, "WORKDIR /workspace", result)
}

func TestLabel_render(t *testing.T) {
	// Act
	result := Label{Key: "project", Value: "agentic-cli"}.Render()

	// Assert
	assert.Equal(t, "LABEL project=agentic-cli", result)
}

func TestEntrypoint_render(t *testing.T) {
	// Act
	result := Entrypoint{Cmd: []string{"/usr/local/bin/entrypoint.sh"}}.Render()

	// Assert
	assert.Equal(t, `ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]`, result)
}
