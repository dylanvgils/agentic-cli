package script

import (
	"os"
	"os/exec"
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

func TestFindScript_found(t *testing.T) {
	// Arrange
	name := "sh"

	// Act
	path := FindScript(name)

	// Assert
	_, err := os.Stat(path)
	require.NoError(t, err, "expected a valid path for %q", name)
}

// TestFindScript_notFound re-executes the test binary as a subprocess to
// capture the os.Exit(1) call without killing the test process.
func TestFindScript_notFound(t *testing.T) {
	if os.Getenv("TEST_FIND_SCRIPT_EXIT") == "1" {
		FindScript("this-binary-does-not-exist-agentic")
		return
	}

	// Arrange
	cmd := exec.Command(os.Args[0], "-test.run=TestFindScript_notFound")
	cmd.Env = append(os.Environ(), "TEST_FIND_SCRIPT_EXIT=1")

	// Act
	out, err := cmd.CombinedOutput()

	// Assert
	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok, "expected ExitError, got %T", err)
	require.Equal(t, 1, exitErr.ExitCode())
	require.Contains(t, string(out), "agentic not found on PATH")
}
