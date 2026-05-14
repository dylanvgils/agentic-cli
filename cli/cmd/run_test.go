package cmd

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureRunContainer replaces the runContainer var with a mock that records
// the RunSpec and tool args passed to it. Returns a getter and a restore func.
func captureRunContainer(t *testing.T) (func() (docker.RunSpec, []string), func()) {
	t.Helper()
	var capturedSpec docker.RunSpec
	var capturedArgs []string

	orig := runContainer
	runContainer = func(rs docker.RunSpec, args []string) error {
		capturedSpec = rs
		capturedArgs = args
		return nil
	}

	get := func() (docker.RunSpec, []string) { return capturedSpec, capturedArgs }
	restore := func() { runContainer = orig }
	return get, restore
}

func TestRunTool_noArgs_printsHelp(t *testing.T) {
	// Arrange
	get, restore := captureRunContainer(t)
	defer restore()

	// Act
	err := runTool(runToolCmd, []string{})

	// Assert
	require.NoError(t, err)
	rs, _ := get()
	assert.Empty(t, rs.Image, "RunContainer should not be called when no args given")
}

func TestRunTool_buildsImageName(t *testing.T) {
	// Arrange
	get, restore := captureRunContainer(t)
	defer restore()

	// Act
	err := runTool(runToolCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	rs, toolArgs := get()
	assert.Equal(t, "agentic-claude", rs.Image)
	assert.Empty(t, toolArgs)
}

func TestRunTool_passesToolArgs(t *testing.T) {
	// Arrange
	get, restore := captureRunContainer(t)
	defer restore()

	// Act
	err := runTool(runToolCmd, []string{"claude", "--dangerously-skip-permissions"})

	// Assert
	require.NoError(t, err)
	_, toolArgs := get()
	assert.Equal(t, []string{"--dangerously-skip-permissions"}, toolArgs)
}

func TestRunTool_dashDash_setsSkipEntrypoint(t *testing.T) {
	// Arrange
	get, restore := captureRunContainer(t)
	defer restore()

	// Act
	err := runTool(runToolCmd, []string{"claude", "--", "bash", "-c", "echo hi"})

	// Assert
	require.NoError(t, err)
	rs, toolArgs := get()
	assert.True(t, rs.SkipEntrypoint)
	assert.Equal(t, []string{"bash", "-c", "echo hi"}, toolArgs)
}

func TestRunTool_dashDash_noTrailingArgs(t *testing.T) {
	// Arrange
	get, restore := captureRunContainer(t)
	defer restore()

	// Act
	err := runTool(runToolCmd, []string{"claude", "--"})

	// Assert
	require.NoError(t, err)
	rs, toolArgs := get()
	assert.True(t, rs.SkipEntrypoint)
	assert.Empty(t, toolArgs)
}

func TestRunTool_extraVolumes(t *testing.T) {
	// Arrange
	get, restore := captureRunContainer(t)
	defer restore()
	origVolumes := extraVolumes
	extraVolumes = []string{"/host:/container"}
	defer func() { extraVolumes = origVolumes }()

	// Act
	err := runTool(runToolCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	rs, _ := get()
	assert.Equal(t, []string{"/host:/container"}, rs.Volumes)
}

func TestRunTool_toolHome(t *testing.T) {
	// Arrange
	get, restore := captureRunContainer(t)
	defer restore()
	origHome := toolHome
	toolHome = "/custom/home"
	defer func() { toolHome = origHome }()

	// Act
	err := runTool(runToolCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	rs, _ := get()
	assert.Equal(t, "/custom/home", rs.ToolHome)
}
