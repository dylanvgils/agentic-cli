package script

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindScriptSafe_found(t *testing.T) {
	// Arrange
	name := "sh"

	// Act
	path := findScriptSafe(name)

	// Assert
	_, err := os.Stat(path)
	require.NoError(t, err, "expected a valid path for %q", name)
}

func TestFindScriptSafe_notFound(t *testing.T) {
	// Arrange
	name := "this-binary-does-not-exist-agentic"

	// Act
	path := findScriptSafe(name)

	// Assert
	require.Empty(t, path)
}
