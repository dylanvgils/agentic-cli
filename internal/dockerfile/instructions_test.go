package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- From ---

func TestFrom_noAs(t *testing.T) {
	// Act
	result := From{Image: "debian:bookworm-slim"}.Render()

	// Assert
	assert.Equal(t, "FROM debian:bookworm-slim", result)
}

func TestFrom_withAs(t *testing.T) {
	// Act
	result := From{Image: "debian:bookworm-slim", As: "base"}.Render()

	// Assert
	assert.Equal(t, "FROM debian:bookworm-slim AS base", result)
}

// --- Arg ---

func TestArg_noDefault(t *testing.T) {
	// Act
	result := Arg{Key: "HOST_UID"}.Render()

	// Assert
	assert.Equal(t, "ARG HOST_UID", result)
}

func TestArg_withDefault(t *testing.T) {
	// Act
	result := Arg{Key: "NODE_VERSION", Default: "24"}.Render()

	// Assert
	assert.Equal(t, "ARG NODE_VERSION=24", result)
}

// --- Run ---

func TestRun_singleLine(t *testing.T) {
	// Act
	result := Run{Command: "apt-get update"}.Render()

	// Assert
	assert.Equal(t, "RUN apt-get update", result)
}

func TestRun_blocks_noComment(t *testing.T) {
	// Arrange
	run := Run{Blocks: []Block{
		{Lines: []string{"apt-get update"}},
		{Lines: []string{"apt-get install curl", "wget"}},
		{Lines: []string{"rm -rf /var/lib/apt/lists/*"}},
	}}

	// Act
	result := run.Render()

	// Assert
	assert.Equal(t, "RUN apt-get update \\\n  && apt-get install curl \\\n  wget \\\n  && rm -rf /var/lib/apt/lists/*", result)
}

func TestRun_blocks_withComment(t *testing.T) {
	// Arrange
	run := Run{Blocks: []Block{
		{Comment: "Update package list", Lines: []string{"apt-get update"}},
		{Comment: "Install packages", Lines: []string{"apt-get install curl"}},
	}}

	// Act
	result := run.Render()

	// Assert
	assert.Equal(t, "RUN \\\n  # Update package list\n  apt-get update \\\n  \\\n  # Install packages\n  && apt-get install curl", result)
}

func TestRun_multiLine(t *testing.T) {
	// Act
	result := Run{Lines: []string{"apt-get update", "&& apt-get install curl"}}.Render()

	// Assert
	assert.Equal(t, "RUN apt-get update \\\n  && apt-get install curl", result)
}


// --- Env ---

func TestEnv_render(t *testing.T) {
	// Act
	result := Env{Key: "DEBIAN_FRONTEND", Value: "noninteractive"}.Render()

	// Assert
	assert.Equal(t, "ENV DEBIAN_FRONTEND=noninteractive", result)
}

// --- Shell ---

func TestShell_render(t *testing.T) {
	// Act
	result := Shell{Form: []string{"/bin/bash", "-o", "pipefail", "-c"}}.Render()

	// Assert
	assert.Equal(t, `SHELL ["/bin/bash", "-o", "pipefail", "-c"]`, result)
}

// --- User ---

func TestUser_render(t *testing.T) {
	// Act
	result := User{Name: "claude"}.Render()

	// Assert
	assert.Equal(t, "USER claude", result)
}

// --- Workdir ---

func TestWorkdir_render(t *testing.T) {
	// Act
	result := Workdir{Path: "/workspace"}.Render()

	// Assert
	assert.Equal(t, "WORKDIR /workspace", result)
}

// --- Label ---

func TestLabel_render(t *testing.T) {
	// Act
	result := Label{Key: "project", Value: "agentic-cli"}.Render()

	// Assert
	assert.Equal(t, "LABEL project=agentic-cli", result)
}

// --- Entrypoint ---

func TestEntrypoint_render(t *testing.T) {
	// Act
	result := Entrypoint{Cmd: []string{"/usr/local/bin/entrypoint.sh"}}.Render()

	// Assert
	assert.Equal(t, `ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]`, result)
}

// --- Located ---

func TestLocated_render(t *testing.T) {
	// Arrange
	located := Located{Source: "internal/tools/bases.go:42", Inst: Env{Key: "FOO", Value: "bar"}}

	// Act
	result := located.Render()

	// Assert
	assert.Equal(t, "# internal/tools/bases.go:42\nENV FOO=bar", result)
}

func TestLocated_emptySource(t *testing.T) {
	// Arrange
	located := Located{Inst: Env{Key: "FOO", Value: "bar"}}

	// Act
	result := located.Render()

	// Assert
	assert.Equal(t, "ENV FOO=bar", result)
}

func TestLocate_capturesSource(t *testing.T) {
	// Act
	located := Locate(Env{Key: "FOO", Value: "bar"})

	// Assert
	assert.Contains(t, located.Source, "instructions_test.go:")
	assert.IsType(t, Env{}, located.Inst)
}
