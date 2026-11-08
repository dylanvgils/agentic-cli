package resolve

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
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
	rc := &config.AgenticRC{Run: config.RCRun{RCLimits: config.RCLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}}}

	for _, tc := range []struct {
		name  string
		flags sandbox.ResourceLimits
		rc    *config.AgenticRC
		want  sandbox.ResourceLimits
	}{
		{
			name: "rc fills empty flags",
			rc:   rc,
			want: sandbox.ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"},
		},
		{
			name:  "flag takes precedence over rc",
			flags: sandbox.ResourceLimits{PidsLimit: "1024", CPUs: "4", Memory: "4g"},
			rc:    rc,
			want:  sandbox.ResourceLimits{PidsLimit: "1024", CPUs: "4", Memory: "4g"},
		},
		{
			name:  "partial flags rc fills rest",
			flags: sandbox.ResourceLimits{PidsLimit: "1024"},
			rc:    rc,
			want:  sandbox.ResourceLimits{PidsLimit: "1024", CPUs: "2", Memory: "2g"},
		},
		{
			name: "falls back to hardcoded default when nothing else set",
			rc:   &config.AgenticRC{},
			want: sandbox.ResourceLimits{PidsLimit: sandbox.DefaultPidsLimit, CPUs: sandbox.DefaultCPUs, Memory: sandbox.DefaultMemory},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := ResourceLimitsFor(tc.flags, tc.rc)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
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
