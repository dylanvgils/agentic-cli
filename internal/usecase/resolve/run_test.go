package resolve

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/stretchr/testify/assert"
)

func TestVolumes(t *testing.T) {
	t.Run("ordering", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Mounts: []string{"rcvol:/mnt/rc"}}}

		// Act
		result := Volumes([]string{"tool:/mnt/tool"}, []string{"flagvol:/mnt/flag"}, rc)

		// Assert
		assert.Equal(t, []string{
			"tool:/mnt/tool",
			"flagvol:/mnt/flag",
			"rcvol:/mnt/rc",
		}, result)
	})

	t.Run("no sources returns empty", func(t *testing.T) {
		// Act
		result := Volumes(nil, nil, &config.AgenticRC{})

		// Assert
		assert.Empty(t, result)
	})

	t.Run("does not mutate tool mounts", func(t *testing.T) {
		// Arrange
		toolMounts := []string{"tool:/mnt/tool"}

		// Act
		result := Volumes(toolMounts, []string{"extra:/mnt/extra"}, &config.AgenticRC{})

		// Assert
		assert.Len(t, toolMounts, 1, "original toolMounts slice should not be modified")
		assert.Len(t, result, 2)
	})

	t.Run("user bind mounts default to read-only", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Mounts: []string{"/host/rc:/mnt/rc"}}}

		// Act
		result := Volumes(nil, []string{"/host/flag:/mnt/flag"}, rc)

		// Assert
		assert.Equal(t, []string{"/host/flag:/mnt/flag:ro", "/host/rc:/mnt/rc:ro"}, result)
	})

	t.Run("tool bind mounts stay read-write", func(t *testing.T) {
		// Act
		result := Volumes([]string{"$TOOL_HOME/claude/data:$CONTAINER_HOME/.claude"}, nil, &config.AgenticRC{})

		// Assert
		assert.Equal(t, []string{"$TOOL_HOME/claude/data:$CONTAINER_HOME/.claude"}, result)
	})
}

func TestSecrets(t *testing.T) {
	t.Run("ordering", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Secrets: []string{"rctoken:/tmp/rc"}}}

		// Act
		result := Secrets([]string{"flagtoken:/tmp/flag"}, rc)

		// Assert
		assert.Equal(t, []string{
			"flagtoken:/tmp/flag",
			"rctoken:/tmp/rc",
		}, result)
	})

	t.Run("all empty returns nil", func(t *testing.T) {
		// Act
		result := Secrets(nil, &config.AgenticRC{})

		// Assert
		assert.Nil(t, result)
	})
}

func TestToolArgs(t *testing.T) {
	rc := &config.AgenticRC{Run: config.RCRun{Args: map[string][]string{
		"copilot": {"--from-rc"},
		"claude":  {"--other"},
	}}}

	t.Run("puts the tool's rc args before the command line's", func(t *testing.T) {
		// Act
		result := ToolArgs("copilot", []string{"--from-cli"}, false, rc)

		// Assert
		assert.Equal(t, []string{"--from-rc", "--from-cli"}, result)
	})

	t.Run("skipping the entrypoint drops the rc args", func(t *testing.T) {
		// Act
		result := ToolArgs("copilot", []string{"bash"}, true, rc)

		// Assert
		assert.Equal(t, []string{"bash"}, result)
	})

	t.Run("does not mutate the rc args", func(t *testing.T) {
		// Act
		_ = ToolArgs("copilot", []string{"--from-cli"}, false, rc)

		// Assert
		assert.Equal(t, []string{"--from-rc"}, rc.Run.Args["copilot"])
	})
}

func TestEnv(t *testing.T) {
	t.Run("ordering, flag wins over rc on duplicate key", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Env: []string{"FOO=fromrc"}}}

		// Act
		result := Env([]string{"FOO=fromflag"}, rc)

		// Assert
		assert.Equal(t, []string{"FOO=fromrc", "FOO=fromflag"}, result)
	})

	t.Run("no sources returns empty", func(t *testing.T) {
		// Act
		result := Env(nil, &config.AgenticRC{})

		// Assert
		assert.Empty(t, result)
	})

	t.Run("does not mutate rc env slice", func(t *testing.T) {
		// Arrange
		rcEnv := []string{"FOO=bar"}
		rc := &config.AgenticRC{Run: config.RCRun{Env: rcEnv}}

		// Act
		result := Env([]string{"BAZ=qux"}, rc)

		// Assert
		assert.Len(t, rcEnv, 1, "original rc env slice should not be modified")
		assert.Len(t, result, 2)
	})
}

func TestResourceLimitsFor(t *testing.T) {
	t.Run("rc fills empty flags", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{RCLimits: config.RCLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}}}

		// Act
		result := ResourceLimitsFor(sandbox.ResourceLimits{}, rc)

		// Assert
		assert.Equal(t, "512", result.PidsLimit)
		assert.Equal(t, "2", result.CPUs)
		assert.Equal(t, "2g", result.Memory)
	})

	t.Run("flag takes precedence over rc", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{RCLimits: config.RCLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}}}

		// Act
		result := ResourceLimitsFor(sandbox.ResourceLimits{PidsLimit: "1024", CPUs: "4", Memory: "4g"}, rc)

		// Assert
		assert.Equal(t, "1024", result.PidsLimit)
		assert.Equal(t, "4", result.CPUs)
		assert.Equal(t, "4g", result.Memory)
	})

	t.Run("partial flags rc fills rest", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{RCLimits: config.RCLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}}}

		// Act
		result := ResourceLimitsFor(sandbox.ResourceLimits{PidsLimit: "1024"}, rc)

		// Assert
		assert.Equal(t, "1024", result.PidsLimit)
		assert.Equal(t, "2", result.CPUs)
		assert.Equal(t, "2g", result.Memory)
	})

	t.Run("falls back to hardcoded default when nothing else set", func(t *testing.T) {
		// Act
		result := ResourceLimitsFor(sandbox.ResourceLimits{}, &config.AgenticRC{})

		// Assert - always resolved, never left empty
		assert.Equal(t, sandbox.DefaultPidsLimit, result.PidsLimit)
		assert.Equal(t, sandbox.DefaultCPUs, result.CPUs)
		assert.Equal(t, sandbox.DefaultMemory, result.Memory)
	})
}

func TestRuntime(t *testing.T) {
	rc := &config.AgenticRC{Run: config.RCRun{Runtime: "microsandbox"}}

	t.Run("flag takes precedence over rc", func(t *testing.T) {
		// Act
		runtime := Runtime(sandbox.RuntimeDocker, rc)

		// Assert
		assert.Equal(t, sandbox.RuntimeDocker, runtime)
	})

	t.Run("rc fills an unset flag", func(t *testing.T) {
		// Act
		runtime := Runtime("", rc)

		// Assert
		assert.Equal(t, sandbox.RuntimeMicrosandbox, runtime)
	})

	t.Run("defaults to docker", func(t *testing.T) {
		// Act
		runtime := Runtime("", &config.AgenticRC{})

		// Assert
		assert.Equal(t, sandbox.RuntimeDocker, runtime)
	})
}
