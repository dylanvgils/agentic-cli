package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runPrune(t *testing.T) {
	t.Run("force clears the build cache without asking", func(t *testing.T) {
		// Arrange
		logBuf := stubErrLog(t)
		cachePruned := false
		stubDocker(t, &fakeDocker{pruneBuildCache: func() (string, error) {
			cachePruned = true
			return "", nil
		}})

		// Act
		err := runPrune(newTestPruneCmd(true), nil)

		// Assert
		require.NoError(t, err)
		assert.True(t, cachePruned)
		assert.Contains(t, logBuf.String(), "agentic: pruning unused resources")
		assert.NotContains(t, logBuf.String(), "[y/N]")
	})

	t.Run("without force a declined prompt keeps the build cache", func(t *testing.T) {
		// Arrange - empty stdin declines whether or not the test runs on a terminal
		stubStdin(t, "")
		stubErrLog(t)
		cachePruned := false
		stubDocker(t, &fakeDocker{pruneBuildCache: func() (string, error) {
			cachePruned = true
			return "", nil
		}})

		// Act
		err := runPrune(newTestPruneCmd(false), nil)

		// Assert
		require.NoError(t, err)
		assert.False(t, cachePruned)
	})
}
