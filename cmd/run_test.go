package cmd

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
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
		t.Chdir(t.TempDir())
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
}

func Test_buildRunSpec(t *testing.T) {
	stubEnsureNamedVolumes(t, func([]string, string, string, string) error { return nil })
	stubEnsureNetwork(t, func() error { return nil })

	t.Run("volumes wired", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		orig := extraVolumes
		extraVolumes = []string{"/host:/container"}
		defer func() { extraVolumes = orig }()
		args := parsedArgs{toolName: "claude", imageName: "agentic-claude"}

		// Act
		rs, err := buildRunSpec(args, tools.Configs["claude"], &config.AgenticRC{}, "")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, rs.Volumes, "/host:/container")
	})

	t.Run("secrets wired", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		orig := flagSecrets
		flagSecrets = []string{"mytoken:/tmp/token"}
		defer func() { flagSecrets = orig }()
		args := parsedArgs{toolName: "claude", imageName: "agentic-claude"}

		// Act
		rs, err := buildRunSpec(args, tools.Configs["claude"], &config.AgenticRC{}, "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"mytoken:/tmp/token"}, rs.Secrets)
	})

	t.Run("resource limits wired", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		rc := &config.AgenticRC{Run: config.RCRun{PidsLimit: "512", CPUs: "2", Memory: "2g"}}
		args := parsedArgs{toolName: "claude", imageName: "agentic-claude"}

		// Act
		rs, err := buildRunSpec(args, tools.Configs["claude"], rc, "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "512", rs.PidsLimit)
		assert.Equal(t, "2", rs.CPUs)
		assert.Equal(t, "2g", rs.Memory)
	})

	t.Run("dry run wired", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		orig := dryRun
		dryRun = true
		defer func() { dryRun = orig }()
		args := parsedArgs{toolName: "claude", imageName: "agentic-claude"}

		// Act
		rs, err := buildRunSpec(args, tools.Configs["claude"], &config.AgenticRC{}, "")

		// Assert
		require.NoError(t, err)
		assert.True(t, rs.DryRun)
	})

	t.Run("tmpfs mounts wired", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		args := parsedArgs{toolName: "claude", imageName: "agentic-claude"}

		// Act
		rs, err := buildRunSpec(args, tools.Configs["claude"], &config.AgenticRC{}, "")

		// Assert
		require.NoError(t, err)
		assert.NotEmpty(t, rs.TmpfsMounts)
	})

	t.Run("tool home wired", func(t *testing.T) {
		// Arrange
		customHome := t.TempDir()
		orig := toolHome
		toolHome = customHome
		defer func() { toolHome = orig }()
		args := parsedArgs{toolName: "claude", imageName: "agentic-claude"}

		// Act
		rs, err := buildRunSpec(args, tools.Configs["claude"], &config.AgenticRC{}, "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, customHome, rs.ToolHome)
	})

	t.Run("skip entrypoint wired", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		args := parsedArgs{toolName: "claude", imageName: "agentic-claude", skipEntrypoint: true}

		// Act
		rs, err := buildRunSpec(args, tools.Configs["claude"], &config.AgenticRC{}, "")

		// Assert
		require.NoError(t, err)
		assert.True(t, rs.SkipEntrypoint)
	})

	t.Run("ensure named volumes error propagates", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		stubEnsureNamedVolumes(t, func([]string, string, string, string) error { return fmt.Errorf("volume error") })
		args := parsedArgs{toolName: "claude", imageName: "agentic-claude"}

		// Act
		_, err := buildRunSpec(args, tools.Configs["claude"], &config.AgenticRC{}, "")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "volume error")
	})

	t.Run("ensure network error propagates", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		stubEnsureNetwork(t, func() error { return fmt.Errorf("network error") })
		args := parsedArgs{toolName: "claude", imageName: "agentic-claude"}

		// Act
		_, err := buildRunSpec(args, tools.Configs["claude"], &config.AgenticRC{}, "")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "network error")
	})
}

func TestRequireImage(t *testing.T) {
	t.Run("image exists returns nil", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{Image: "agentic-claude"}, nil)

		// Act
		err := requireImage("agentic-claude", "claude")

		// Assert
		require.NoError(t, err)
	})

	t.Run("inspect error propagates", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, fmt.Errorf("docker daemon not running"))

		// Act
		err := requireImage("agentic-claude", "claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	t.Run("passes tool filter to list", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)
		var got []docker.ImageFilter
		stubListAllImages(t, func(f ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			got = f
			return nil, nil
		})

		// Act
		_ = requireImage("agentic-claude", "claude")

		// Assert
		assert.Equal(t, []docker.ImageFilter{docker.ToolFilter("claude")}, got)
	})

	t.Run("no alternatives suggests build", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) { return nil, nil })

		// Act
		err := requireImage("agentic-claude", "claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "agentic-claude")
		assert.Contains(t, err.Error(), "agentic build claude")
	})

	t.Run("alternative namespace suggests --namespace", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{{Namespace: "myproject"}}, nil
		})

		// Act
		err := requireImage("agentic-claude", "claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "agentic-claude")
		assert.Contains(t, err.Error(), "myproject")
		assert.Contains(t, err.Error(), "--namespace")
	})

	t.Run("multiple alternative namespaces lists all", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Namespace: "myproject"},
				{Namespace: "work"},
			}, nil
		})

		// Act
		err := requireImage("agentic-claude", "claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "myproject")
		assert.Contains(t, err.Error(), "work")
		assert.Contains(t, err.Error(), "--namespace")
	})

	t.Run("single namespace uses singular noun", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{{Namespace: "myproject"}}, nil
		})

		// Act
		err := requireImage("agentic-claude", "claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "namespace ")
		assert.NotContains(t, err.Error(), "namespaces ")
	})

	t.Run("multiple namespaces uses plural noun", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Namespace: "myproject"},
				{Namespace: "work"},
			}, nil
		})

		// Act
		err := requireImage("agentic-claude", "claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "namespaces ")
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
		rc := &config.AgenticRC{Run: config.RCRun{ExtraMounts: []string{"rcvol:/mnt/rc"}}}

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
		rc := &config.AgenticRC{Run: config.RCRun{Secrets: []string{"rctoken:/tmp/rc"}}}

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
		rc := &config.AgenticRC{Run: config.RCRun{PidsLimit: "512", CPUs: "2", Memory: "2g"}}

		// Act
		result := resolveResourceLimits("", "", "", rc)

		// Assert
		assert.Equal(t, "512", result.pidsLimit)
		assert.Equal(t, "2", result.cpus)
		assert.Equal(t, "2g", result.memory)
	})

	t.Run("flag takes precedence over rc", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{PidsLimit: "512", CPUs: "2", Memory: "2g"}}

		// Act
		result := resolveResourceLimits("1024", "4", "4g", rc)

		// Assert
		assert.Equal(t, "1024", result.pidsLimit)
		assert.Equal(t, "4", result.cpus)
		assert.Equal(t, "4g", result.memory)
	})

	t.Run("partial flags rc fills rest", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{PidsLimit: "512", CPUs: "2", Memory: "2g"}}

		// Act
		result := resolveResourceLimits("1024", "", "", rc)

		// Assert
		assert.Equal(t, "1024", result.pidsLimit)
		assert.Equal(t, "2", result.cpus)
		assert.Equal(t, "2g", result.memory)
	})
}
