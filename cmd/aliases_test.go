package cmd

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunAliases(t *testing.T) {
	t.Run("prints preamble", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)

		// Act
		out := captureStdout(t, func() {
			err := runAliases(aliasesCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "# agentic tool aliases - source with: source <(agentic aliases)")
	})

	t.Run("not built tools emit nothing after preamble", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)

		// Act
		out := captureStdout(t, func() {
			err := runAliases(aliasesCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.NotContains(t, out, "alias ")
	})

	t.Run("built tools emit alias lines", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{Image: "agentic-claude", ID: "abc123"}, nil)

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

	t.Run("docker error propagates", func(t *testing.T) {
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
	})
}
