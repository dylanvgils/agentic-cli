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

func TestDelegate_success(t *testing.T) {
	// Arrange - use a small inline script that exits 0
	f, err := os.CreateTemp(t.TempDir(), "script-*.sh")
	require.NoError(t, err)
	_, err = f.WriteString("#!/usr/bin/env bash\nexit 0\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	require.NoError(t, os.Chmod(f.Name(), 0o755))

	// Act + Assert - no error expected; LookPath accepts absolute paths
	err = Delegate(f.Name(), []string{})
	require.NoError(t, err)
}

func TestDelegate_forwardsArgs(t *testing.T) {
	// Arrange - script writes its args to a temp file
	dir := t.TempDir()
	out := dir + "/args.txt"
	f, err := os.CreateTemp(dir, "script-*.sh")
	require.NoError(t, err)
	_, err = f.WriteString("#!/usr/bin/env bash\necho \"$@\" > " + out + "\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	require.NoError(t, os.Chmod(f.Name(), 0o755))

	// Act
	err = Delegate(f.Name(), []string{"foo", "bar"})
	require.NoError(t, err)

	// Assert
	got, err := os.ReadFile(out)
	require.NoError(t, err)
	require.Equal(t, "foo bar\n", string(got))
}

func TestDelegate_nonZeroExit(t *testing.T) {
	if os.Getenv("TEST_DELEGATE_EXIT") == "1" {
		f, _ := os.CreateTemp(t.TempDir(), "script-*.sh")
		_, _ = f.WriteString("#!/usr/bin/env bash\nexit 42\n")
		_ = f.Close()
		_ = os.Chmod(f.Name(), 0o755)
		_ = Delegate(f.Name(), []string{})
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestDelegate_nonZeroExit")
	cmd.Env = append(os.Environ(), "TEST_DELEGATE_EXIT=1")

	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok, "expected ExitError, got %T", err)
	require.Equal(t, 42, exitErr.ExitCode())
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
