package cmd

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunAliases_printsPreamble(t *testing.T) {
	// Arrange
	restore := stubInspectImage(t, nil, nil)
	defer restore()

	// Act
	out := captureStdout(t, func() {
		err := runAliases(aliasesCmd, []string{})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "# agentic tool aliases - source with: source <(agentic aliases)")
}

func TestRunAliases_notBuiltTools_emitNothingAfterPreamble(t *testing.T) {
	// Arrange
	restore := stubInspectImage(t, nil, nil)
	defer restore()

	// Act
	out := captureStdout(t, func() {
		err := runAliases(aliasesCmd, []string{})
		require.NoError(t, err)
	})

	// Assert
	assert.NotContains(t, out, "alias ")
}

func TestRunAliases_builtTools_emitAliasLines(t *testing.T) {
	// Arrange
	restore := stubInspectImage(t, &docker.ImageInfo{Image: "agentic-claude", ID: "abc123"}, nil)
	defer restore()

	// Act
	out := captureStdout(t, func() {
		err := runAliases(aliasesCmd, []string{})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "alias claude='agentic run claude'")
	assert.Contains(t, out, "alias copilot='agentic run copilot'")
	assert.Contains(t, out, "alias opencode='agentic run opencode'")
}

func TestRunAliases_dockerError_propagates(t *testing.T) {
	// Arrange
	orig := inspectImage
	inspectImage = func(_ string) (*docker.ImageInfo, error) {
		return nil, fmt.Errorf("docker daemon not running")
	}
	defer func() { inspectImage = orig }()

	// Act
	err := runAliases(aliasesCmd, []string{})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker daemon not running")
}
