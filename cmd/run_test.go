package cmd

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureRunContainer replaces runContainer and ensureNamedVolumes with stubs
// that record the RunSpec and tool args. Returns a getter and a restore func.
func captureRunContainer(t *testing.T) (func() (docker.RunSpec, []string), func()) {
	t.Helper()
	var capturedSpec docker.RunSpec
	var capturedArgs []string

	origRun := runContainer
	runContainer = func(rs docker.RunSpec, args []string) error {
		capturedSpec = rs
		capturedArgs = args
		return nil
	}

	origEnsure := ensureNamedVolumes
	ensureNamedVolumes = func(volumes []string, toolHome, containerHome string) error {
		return nil
	}

	get := func() (docker.RunSpec, []string) { return capturedSpec, capturedArgs }
	restore := func() {
		runContainer = origRun
		ensureNamedVolumes = origEnsure
	}
	return get, restore
}

// withTempToolHome sets toolHome to a temp dir and pre-trusts the directories
// that tests run in (os.TempDir() covers t.Chdir paths; cwd covers tests that
// don't chdir).
func withTempToolHome(t *testing.T) {
	t.Helper()
	homeDir := t.TempDir()
	cwd, err := os.Getwd()
	require.NoError(t, err)
	cfg := &config.CliConfig{TrustedDirs: []string{os.TempDir(), cwd}}
	require.NoError(t, cfg.Save(homeDir))
	orig := toolHome
	toolHome = homeDir
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

func TestRunTool_unknownTool_returnsError(t *testing.T) {
	// Act
	err := runTool(runToolCmd, []string{"bogus"})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bogus")
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
		[]byte("extra_mounts=myvolume:/mnt/data\n"),
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
		[]byte("pids_limit=512\ncpus=2\nmemory=2g\n"),
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

func TestRunTool_agenticExtraMountsEnv(t *testing.T) {
	// Arrange
	withTempToolHome(t)
	t.Chdir(t.TempDir())
	t.Setenv("AGENTIC_EXTRA_MOUNTS", "vol:/mnt/data")
	get, restore := captureRunContainer(t)
	defer restore()

	// Act
	err := runTool(runToolCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	rs, _ := get()
	assert.Contains(t, rs.Volumes, "vol:/mnt/data")
	// env mounts appear before CLI -v flags (after tool defaults)
	defaultEnd := 2 // last tool-default index
	envIdx := -1
	for i, v := range rs.Volumes {
		if v == "vol:/mnt/data" {
			envIdx = i
		}
	}
	assert.Greater(t, envIdx, defaultEnd, "env mount should come after tool defaults")
}

func TestRunTool_agenticExtraMountsEnv_empty(t *testing.T) {
	// Arrange
	withTempToolHome(t)
	t.Chdir(t.TempDir())
	t.Setenv("AGENTIC_EXTRA_MOUNTS", "")
	get, restore := captureRunContainer(t)
	defer restore()

	// Act
	err := runTool(runToolCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	rs, _ := get()
	// Only tool-default mounts; no empty entry added
	assert.Equal(t, []string{
		"$PWD:/workspace",
		"$TOOL_HOME/claude/data:$CONTAINER_HOME/.claude",
		"$TOOL_HOME/claude/.claude.json:$CONTAINER_HOME/.claude.json",
	}, rs.Volumes)
}

func TestRunTool_flagSecrets(t *testing.T) {
	// Arrange
	withTempToolHome(t)
	t.Chdir(t.TempDir())
	get, restore := captureRunContainer(t)
	defer restore()
	origSecrets := flagSecrets
	flagSecrets = []string{"mytoken:/tmp/token"}
	defer func() { flagSecrets = origSecrets }()

	// Act
	err := runTool(runToolCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	rs, _ := get()
	assert.Equal(t, []string{"mytoken:/tmp/token"}, rs.Secrets)
}

func TestRunTool_agenticrcSecretsAppended(t *testing.T) {
	// Arrange
	withTempToolHome(t)
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile(
		dir+"/.agenticrc",
		[]byte("secrets=rctoken:/tmp/rc_token\n"),
		0644,
	))
	get, restore := captureRunContainer(t)
	defer restore()
	origSecrets := flagSecrets
	flagSecrets = []string{"flagtoken:/tmp/flag_token"}
	defer func() { flagSecrets = origSecrets }()

	// Act
	err := runTool(runToolCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	rs, _ := get()
	assert.Equal(t, []string{"flagtoken:/tmp/flag_token", "rctoken:/tmp/rc_token"}, rs.Secrets)
}

func TestRunTool_agenticExtraSecretsEnv(t *testing.T) {
	// Arrange
	withTempToolHome(t)
	t.Chdir(t.TempDir())
	t.Setenv("AGENTIC_SECRETS", "envtoken:/tmp/env_token")
	get, restore := captureRunContainer(t)
	defer restore()

	// Act
	err := runTool(runToolCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	rs, _ := get()
	assert.Contains(t, rs.Secrets, "envtoken:/tmp/env_token")
}

func TestParseArgs_toolNameAndImageName(t *testing.T) {
	// Act
	result, err := parseArgs([]string{"claude"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "claude", result.toolName)
	assert.Equal(t, "agentic-claude", result.imageName)
	assert.Empty(t, result.toolArgs)
	assert.False(t, result.skipEntrypoint)
}

func TestParseArgs_toolArgs(t *testing.T) {
	// Act
	result, err := parseArgs([]string{"claude", "--flag", "value"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"--flag", "value"}, result.toolArgs)
	assert.False(t, result.skipEntrypoint)
}

func TestParseArgs_dashDash_setsSkipEntrypoint(t *testing.T) {
	// Act
	result, err := parseArgs([]string{"claude", "--", "bash", "-c", "echo hi"})

	// Assert
	require.NoError(t, err)
	assert.True(t, result.skipEntrypoint)
	assert.Equal(t, []string{"bash", "-c", "echo hi"}, result.toolArgs)
}

func TestParseArgs_dashDash_noTrailingArgs(t *testing.T) {
	// Act
	result, err := parseArgs([]string{"claude", "--"})

	// Assert
	require.NoError(t, err)
	assert.True(t, result.skipEntrypoint)
	assert.Empty(t, result.toolArgs)
}

func TestParseArgs_unknownTool_returnsError(t *testing.T) {
	// Act
	_, err := parseArgs([]string{"bogus"})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bogus")
}

func TestCollectVolumes_ordering(t *testing.T) {
	// Arrange
	t.Setenv("AGENTIC_EXTRA_MOUNTS", "envvol:/mnt/env")
	rc := &config.AgenticRC{ExtraMounts: []string{"rcvol:/mnt/rc"}}

	// Act
	result := collectVolumes([]string{"tool:/mnt/tool"}, []string{"flagvol:/mnt/flag"}, rc)

	// Assert
	assert.Equal(t, []string{
		"tool:/mnt/tool",
		"envvol:/mnt/env",
		"flagvol:/mnt/flag",
		"rcvol:/mnt/rc",
	}, result)
}

func TestCollectVolumes_emptyEnv_skipped(t *testing.T) {
	// Arrange
	t.Setenv("AGENTIC_EXTRA_MOUNTS", "")
	rc := &config.AgenticRC{}

	// Act
	result := collectVolumes([]string{"tool:/mnt/tool"}, nil, rc)

	// Assert
	assert.Equal(t, []string{"tool:/mnt/tool"}, result)
}

func TestCollectVolumes_noSources_returnsEmpty(t *testing.T) {
	// Arrange
	t.Setenv("AGENTIC_EXTRA_MOUNTS", "")
	rc := &config.AgenticRC{}

	// Act
	result := collectVolumes(nil, nil, rc)

	// Assert
	assert.Empty(t, result)
}

func TestCollectVolumes_doesNotMutateToolMounts(t *testing.T) {
	// Arrange
	t.Setenv("AGENTIC_EXTRA_MOUNTS", "")
	toolMounts := []string{"tool:/mnt/tool"}
	rc := &config.AgenticRC{}

	// Act
	result := collectVolumes(toolMounts, []string{"extra:/mnt/extra"}, rc)

	// Assert
	assert.Len(t, toolMounts, 1, "original toolMounts slice should not be modified")
	assert.Len(t, result, 2)
}

func TestCollectSecrets_ordering(t *testing.T) {
	// Arrange
	t.Setenv("AGENTIC_SECRETS", "envtoken:/tmp/env")
	rc := &config.AgenticRC{Secrets: []string{"rctoken:/tmp/rc"}}

	// Act
	result := collectSecrets([]string{"flagtoken:/tmp/flag"}, rc)

	// Assert
	assert.Equal(t, []string{
		"envtoken:/tmp/env",
		"flagtoken:/tmp/flag",
		"rctoken:/tmp/rc",
	}, result)
}

func TestCollectSecrets_emptyEnv_skipped(t *testing.T) {
	// Arrange
	t.Setenv("AGENTIC_SECRETS", "")
	rc := &config.AgenticRC{}

	// Act
	result := collectSecrets([]string{"flagtoken:/tmp/flag"}, rc)

	// Assert
	assert.Equal(t, []string{"flagtoken:/tmp/flag"}, result)
}

func TestCollectSecrets_allEmpty_returnsNil(t *testing.T) {
	// Arrange
	t.Setenv("AGENTIC_SECRETS", "")
	rc := &config.AgenticRC{}

	// Act
	result := collectSecrets(nil, rc)

	// Assert
	assert.Nil(t, result)
}

func TestResolveResourceLimits_rcFillsEmptyFlags(t *testing.T) {
	// Arrange
	rc := &config.AgenticRC{PidsLimit: "512", CPUs: "2", Memory: "2g"}

	// Act
	result := resolveResourceLimits("", "", "", rc)

	// Assert
	assert.Equal(t, "512", result.pidsLimit)
	assert.Equal(t, "2", result.cpus)
	assert.Equal(t, "2g", result.memory)
}

func TestResolveResourceLimits_flagTakesPrecedenceOverRC(t *testing.T) {
	// Arrange
	rc := &config.AgenticRC{PidsLimit: "512", CPUs: "2", Memory: "2g"}

	// Act
	result := resolveResourceLimits("1024", "4", "4g", rc)

	// Assert
	assert.Equal(t, "1024", result.pidsLimit)
	assert.Equal(t, "4", result.cpus)
	assert.Equal(t, "4g", result.memory)
}

func TestResolveResourceLimits_partialFlags_rcFillsRest(t *testing.T) {
	// Arrange
	rc := &config.AgenticRC{PidsLimit: "512", CPUs: "2", Memory: "2g"}

	// Act
	result := resolveResourceLimits("1024", "", "", rc)

	// Assert
	assert.Equal(t, "1024", result.pidsLimit)
	assert.Equal(t, "2", result.cpus)
	assert.Equal(t, "2g", result.memory)
}

func TestRunTool_dryRun(t *testing.T) {
	// Arrange
	withTempToolHome(t)
	get, restore := captureRunContainer(t)
	defer restore()
	origDryRun := dryRun
	dryRun = true
	defer func() { dryRun = origDryRun }()

	// Act
	err := runTool(runToolCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	rs, _ := get()
	assert.True(t, rs.DryRun)
}

func TestRunTool_tmpfsMounts(t *testing.T) {
	// Arrange
	withTempToolHome(t)
	get, restore := captureRunContainer(t)
	defer restore()

	// Act
	err := runTool(runToolCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	rs, _ := get()
	assert.NotEmpty(t, rs.TmpfsMounts)
}

func TestRunTool_toolHome(t *testing.T) {
	// Arrange
	get, restore := captureRunContainer(t)
	defer restore()
	cwd, err := os.Getwd()
	require.NoError(t, err)
	customHome := t.TempDir()
	cfg := &config.CliConfig{TrustedDirs: []string{cwd}}
	require.NoError(t, cfg.Save(customHome))
	orig := toolHome
	toolHome = customHome
	defer func() { toolHome = orig }()

	// Act
	err = runTool(runToolCmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	rs, _ := get()
	assert.Equal(t, customHome, rs.ToolHome)
}

