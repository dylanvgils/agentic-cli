package cmd

import (
	"os"
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

// withTempToolHome sets toolHome to a temp dir for the duration of the test.
func withTempToolHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	orig := toolHome
	toolHome = dir
	t.Cleanup(func() { toolHome = orig })
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
	withTempToolHome(t)
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
	withTempToolHome(t)
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
	withTempToolHome(t)
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
	withTempToolHome(t)
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
	withTempToolHome(t)
	t.Chdir(t.TempDir()) // isolate from workspace .agenticrc
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
	assert.Equal(t, []string{
		"$PWD:/workspace",
		"$TOOL_HOME/claude/data:$CONTAINER_HOME/.claude",
		"$TOOL_HOME/claude/.claude.json:$CONTAINER_HOME/.claude.json",
		"/host:/container",
	}, rs.Volumes)
}

func TestRunTool_agenticrcMountsAppended(t *testing.T) {
	// Arrange
	withTempToolHome(t)
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile(
		dir+"/.agenticrc",
		[]byte("EXTRA_MOUNTS=myvolume:/mnt/data\n"),
		0644,
	))
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
	assert.Equal(t, []string{
		"$PWD:/workspace",
		"$TOOL_HOME/claude/data:$CONTAINER_HOME/.claude",
		"$TOOL_HOME/claude/.claude.json:$CONTAINER_HOME/.claude.json",
		"/host:/container",
		"myvolume:/mnt/data",
	}, rs.Volumes)
}

func TestRunTool_agenticrcResourceLimits(t *testing.T) {
	// Arrange
	withTempToolHome(t)
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile(
		dir+"/.agenticrc",
		[]byte("PIDS_LIMIT=512\nCPUS=2\nMEMORY=2g\n"),
		0644,
	))
	get, restore := captureRunContainer(t)
	defer restore()

	// Act
	err := runTool(runToolCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	rs, _ := get()
	assert.Equal(t, "512", rs.PidsLimit)
	assert.Equal(t, "2", rs.CPUs)
	assert.Equal(t, "2g", rs.Memory)
}

func TestRunTool_toolHome(t *testing.T) {
	// Arrange
	get, restore := captureRunContainer(t)
	defer restore()
	origHome := toolHome
	customHome := t.TempDir()
	toolHome = customHome
	defer func() { toolHome = origHome }()

	// Act
	err := runTool(runToolCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	rs, _ := get()
	assert.Equal(t, customHome, rs.ToolHome)
}
