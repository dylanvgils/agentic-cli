package cmd

import (
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunTool(t *testing.T) {
	t.Run("no args prints help", func(t *testing.T) {
		// Arrange
		get := captureRunContainer(t)

		// Act
		err := runTool(runToolCmd, []string{})

		// Assert
		require.NoError(t, err)
		rs, _ := get()
		assert.Empty(t, rs.Image, "RunContainer should not be called when no args given")
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		err := runTool(runToolCmd, []string{"bogus"})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bogus")
	})

	t.Run("builds image name", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		get := captureRunContainer(t)

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		rs, toolArgs := get()
		assert.Equal(t, "agentic-claude", rs.Image)
		assert.Empty(t, toolArgs)
	})

	t.Run("passes tool args", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		get := captureRunContainer(t)

		// Act
		err := runTool(runToolCmd, []string{"claude", "--dangerously-skip-permissions"})

		// Assert
		require.NoError(t, err)
		_, toolArgs := get()
		assert.Equal(t, []string{"--dangerously-skip-permissions"}, toolArgs)
	})

	t.Run("dash dash sets skip entrypoint", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		get := captureRunContainer(t)

		// Act
		err := runTool(runToolCmd, []string{"claude", "--", "bash", "-c", "echo hi"})

		// Assert
		require.NoError(t, err)
		rs, toolArgs := get()
		assert.True(t, rs.SkipEntrypoint)
		assert.Equal(t, []string{"bash", "-c", "echo hi"}, toolArgs)
	})

	t.Run("dash dash no trailing args", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		get := captureRunContainer(t)

		// Act
		err := runTool(runToolCmd, []string{"claude", "--"})

		// Assert
		require.NoError(t, err)
		rs, toolArgs := get()
		assert.True(t, rs.SkipEntrypoint)
		assert.Empty(t, toolArgs)
	})

	t.Run("extra volumes", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		t.Chdir(t.TempDir())
		get := captureRunContainer(t)
		orig := extraVolumes
		extraVolumes = []string{"/host:/container"}
		defer func() { extraVolumes = orig }()

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
	})

	t.Run("agenticrc mounts appended", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		dir := t.TempDir()
		t.Chdir(dir)
		require.NoError(t, os.WriteFile(
			dir+"/.agenticrc",
			[]byte("extra_mounts=myvolume:/mnt/data\n"),
			0644,
		))
		get := captureRunContainer(t)
		orig := extraVolumes
		extraVolumes = []string{"/host:/container"}
		defer func() { extraVolumes = orig }()

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
	})

	t.Run("agenticrc resource limits", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		dir := t.TempDir()
		t.Chdir(dir)
		require.NoError(t, os.WriteFile(
			dir+"/.agenticrc",
			[]byte("pids_limit=512\ncpus=2\nmemory=2g\n"),
			0644,
		))
		get := captureRunContainer(t)

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		rs, _ := get()
		assert.Equal(t, "512", rs.PidsLimit)
		assert.Equal(t, "2", rs.CPUs)
		assert.Equal(t, "2g", rs.Memory)
	})

	t.Run("agentic extra mounts env", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		t.Chdir(t.TempDir())
		t.Setenv("AGENTIC_EXTRA_MOUNTS", "vol:/mnt/data")
		get := captureRunContainer(t)

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		rs, _ := get()
		assert.Contains(t, rs.Volumes, "vol:/mnt/data")
		defaultEnd := 2
		envIdx := -1
		for i, v := range rs.Volumes {
			if v == "vol:/mnt/data" {
				envIdx = i
			}
		}
		assert.Greater(t, envIdx, defaultEnd, "env mount should come after tool defaults")
	})

	t.Run("agentic extra mounts env empty", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		t.Chdir(t.TempDir())
		t.Setenv("AGENTIC_EXTRA_MOUNTS", "")
		get := captureRunContainer(t)

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		rs, _ := get()
		assert.Equal(t, []string{
			"$PWD:/workspace",
			"$TOOL_HOME/claude/data:$CONTAINER_HOME/.claude",
			"$TOOL_HOME/claude/.claude.json:$CONTAINER_HOME/.claude.json",
		}, rs.Volumes)
	})

	t.Run("flag secrets", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		t.Chdir(t.TempDir())
		get := captureRunContainer(t)
		orig := flagSecrets
		flagSecrets = []string{"mytoken:/tmp/token"}
		defer func() { flagSecrets = orig }()

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		rs, _ := get()
		assert.Equal(t, []string{"mytoken:/tmp/token"}, rs.Secrets)
	})

	t.Run("agenticrc secrets appended", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		dir := t.TempDir()
		t.Chdir(dir)
		require.NoError(t, os.WriteFile(
			dir+"/.agenticrc",
			[]byte("secrets=rctoken:/tmp/rc_token\n"),
			0644,
		))
		get := captureRunContainer(t)
		orig := flagSecrets
		flagSecrets = []string{"flagtoken:/tmp/flag_token"}
		defer func() { flagSecrets = orig }()

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		rs, _ := get()
		assert.Equal(t, []string{"flagtoken:/tmp/flag_token", "rctoken:/tmp/rc_token"}, rs.Secrets)
	})

	t.Run("agentic extra secrets env", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		t.Chdir(t.TempDir())
		t.Setenv("AGENTIC_SECRETS", "envtoken:/tmp/env_token")
		get := captureRunContainer(t)

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		rs, _ := get()
		assert.Contains(t, rs.Secrets, "envtoken:/tmp/env_token")
	})

	t.Run("dry run", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		get := captureRunContainer(t)
		orig := dryRun
		dryRun = true
		defer func() { dryRun = orig }()

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		rs, _ := get()
		assert.True(t, rs.DryRun)
	})

	t.Run("tmpfs mounts", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		get := captureRunContainer(t)

		// Act
		err := runTool(runToolCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		rs, _ := get()
		assert.NotEmpty(t, rs.TmpfsMounts)
	})

	t.Run("tool home", func(t *testing.T) {
		// Arrange
		get := captureRunContainer(t)
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
	})
}

func TestParseArgs(t *testing.T) {
	t.Run("tool name and image name", func(t *testing.T) {
		// Act
		result, err := parseArgs([]string{"claude"}, "agentic")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "claude", result.toolName)
		assert.Equal(t, "agentic-claude", result.imageName)
		assert.Empty(t, result.toolArgs)
		assert.False(t, result.skipEntrypoint)
	})

	t.Run("tool args", func(t *testing.T) {
		// Act
		result, err := parseArgs([]string{"claude", "--flag", "value"}, "agentic")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--flag", "value"}, result.toolArgs)
		assert.False(t, result.skipEntrypoint)
	})

	t.Run("dash dash sets skip entrypoint", func(t *testing.T) {
		// Act
		result, err := parseArgs([]string{"claude", "--", "bash", "-c", "echo hi"}, "agentic")

		// Assert
		require.NoError(t, err)
		assert.True(t, result.skipEntrypoint)
		assert.Equal(t, []string{"bash", "-c", "echo hi"}, result.toolArgs)
	})

	t.Run("dash dash no trailing args", func(t *testing.T) {
		// Act
		result, err := parseArgs([]string{"claude", "--"}, "agentic")

		// Assert
		require.NoError(t, err)
		assert.True(t, result.skipEntrypoint)
		assert.Empty(t, result.toolArgs)
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		_, err := parseArgs([]string{"bogus"}, "agentic")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bogus")
	})
}

func TestCollectVolumes(t *testing.T) {
	t.Run("ordering", func(t *testing.T) {
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
	})

	t.Run("empty env skipped", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_EXTRA_MOUNTS", "")
		rc := &config.AgenticRC{}

		// Act
		result := collectVolumes([]string{"tool:/mnt/tool"}, nil, rc)

		// Assert
		assert.Equal(t, []string{"tool:/mnt/tool"}, result)
	})

	t.Run("no sources returns empty", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_EXTRA_MOUNTS", "")
		rc := &config.AgenticRC{}

		// Act
		result := collectVolumes(nil, nil, rc)

		// Assert
		assert.Empty(t, result)
	})

	t.Run("does not mutate tool mounts", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_EXTRA_MOUNTS", "")
		toolMounts := []string{"tool:/mnt/tool"}
		rc := &config.AgenticRC{}

		// Act
		result := collectVolumes(toolMounts, []string{"extra:/mnt/extra"}, rc)

		// Assert
		assert.Len(t, toolMounts, 1, "original toolMounts slice should not be modified")
		assert.Len(t, result, 2)
	})
}

func TestCollectSecrets(t *testing.T) {
	t.Run("ordering", func(t *testing.T) {
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
	})

	t.Run("empty env skipped", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_SECRETS", "")
		rc := &config.AgenticRC{}

		// Act
		result := collectSecrets([]string{"flagtoken:/tmp/flag"}, rc)

		// Assert
		assert.Equal(t, []string{"flagtoken:/tmp/flag"}, result)
	})

	t.Run("all empty returns nil", func(t *testing.T) {
		// Arrange
		t.Setenv("AGENTIC_SECRETS", "")
		rc := &config.AgenticRC{}

		// Act
		result := collectSecrets(nil, rc)

		// Assert
		assert.Nil(t, result)
	})
}

func TestResolveResourceLimits(t *testing.T) {
	t.Run("rc fills empty flags", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{PidsLimit: "512", CPUs: "2", Memory: "2g"}

		// Act
		result := resolveResourceLimits("", "", "", rc)

		// Assert
		assert.Equal(t, "512", result.pidsLimit)
		assert.Equal(t, "2", result.cpus)
		assert.Equal(t, "2g", result.memory)
	})

	t.Run("flag takes precedence over rc", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{PidsLimit: "512", CPUs: "2", Memory: "2g"}

		// Act
		result := resolveResourceLimits("1024", "4", "4g", rc)

		// Assert
		assert.Equal(t, "1024", result.pidsLimit)
		assert.Equal(t, "4", result.cpus)
		assert.Equal(t, "4g", result.memory)
	})

	t.Run("partial flags rc fills rest", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{PidsLimit: "512", CPUs: "2", Memory: "2g"}

		// Act
		result := resolveResourceLimits("1024", "", "", rc)

		// Assert
		assert.Equal(t, "1024", result.pidsLimit)
		assert.Equal(t, "2", result.cpus)
		assert.Equal(t, "2g", result.memory)
	})
}

