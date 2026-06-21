package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExcludesCLIDependencies guards the reason this binary exists as a
// separate package: the proxy sidecar handles untrusted network traffic, so
// it must never link in the CLI's Docker orchestration, Dockerfile
// generation, or Cobra command tree, even transitively.
func TestExcludesCLIDependencies(t *testing.T) {
	// Arrange
	forbidden := []string{
		"github.com/dylanvgils/agentic-cli/internal/docker",
		"github.com/dylanvgils/agentic-cli/internal/tools",
		"github.com/dylanvgils/agentic-cli/cmd",
		"github.com/spf13/cobra",
	}

	// Act
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	require.NoError(t, err)
	deps := strings.Split(strings.TrimSpace(string(out)), "\n")

	// Assert
	for _, pkg := range forbidden {
		assert.NotContains(t, deps, pkg)
	}
}
