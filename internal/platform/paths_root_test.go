package platform

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLookupBinary_found(t *testing.T) {
	// Arrange
	name := "sh"

	// Act
	path := lookupBinary(name)

	// Assert
	_, err := os.Stat(path)
	require.NoError(t, err, "expected a valid path for %q", name)
}

func TestLookupBinary_notFound(t *testing.T) {
	// Arrange
	name := "this-binary-does-not-exist-agentic"

	// Act
	path := lookupBinary(name)

	// Assert
	require.Empty(t, path)
}

func TestFindRepoRoot_injected(t *testing.T) {
	// Arrange
	original := repoRoot
	repoRoot = "/injected/repo"
	defer func() { repoRoot = original }()

	// Act
	got, err := FindRepoRoot()

	// Assert
	require.NoError(t, err)
	require.Equal(t, "/injected/repo", got)
}
