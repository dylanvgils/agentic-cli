package precedence

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPorts(t *testing.T) {
	t.Run("parses flags, then config", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Ports: []string{"8080:80"}}}

		// Act
		result, err := Ports([]string{"3000"}, rc)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []sandbox.PortMapping{
			{Host: 3000, Guest: 3000},
			{Host: 8080, Guest: 80},
		}, result)
	})

	t.Run("a flag replaces the config port with the same host port", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Ports: []string{"3000:80"}}}

		// Act
		result, err := Ports([]string{"3000"}, rc)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []sandbox.PortMapping{{Host: 3000, Guest: 3000}}, result)
	})

	t.Run("refuses a host port twice in the config", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Ports: []string{
			"3000",
			"3000:80",
		}}}

		// Act
		_, err := Ports(nil, rc)

		// Assert
		assert.EqualError(t, err, `port "3000:80": host port 3000 is forwarded twice`)
	})
}

func TestMounts(t *testing.T) {
	t.Run("lists tool, flag then rc mounts, user bind mounts read-only by default", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Mounts: []string{"/host/rc:/mnt/rc"}}}

		// Act
		result, err := Mounts([]string{"$TOOL_HOME/claude/data:$CONTAINER_HOME/.claude"}, []string{"/host/flag:/mnt/flag"}, rc, "/tool", "/root")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []sandbox.Mount{
			{Source: "/tool/claude/data", Target: "/root/.claude"},
			{Source: "/host/flag", Target: "/mnt/flag", ReadOnly: true},
			{Source: "/host/rc", Target: "/mnt/rc", ReadOnly: true},
		}, result)
	})

	t.Run("returns an invalid mount's error", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Mounts: []string{"/host/only"}}}

		// Act
		_, err := Mounts(nil, nil, rc, "/tool", "/root")

		// Assert
		assert.EqualError(t, err, `invalid volume "/host/only": expected host:container[:options]`)
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
	rc := &config.AgenticRC{Run: config.RCRun{RCLimits: config.RCLimits{PidsLimit: "512", CPUs: "2"}}}

	for _, tc := range []struct {
		name  string
		flags sandbox.ResourceLimits
		rc    *config.AgenticRC
		want  sandbox.ResourceLimits
	}{
		{
			name:  "flag wins over rc, which wins over the default",
			flags: sandbox.ResourceLimits{PidsLimit: "1024"},
			rc:    rc,
			want:  sandbox.ResourceLimits{PidsLimit: "1024", CPUs: "2", Memory: sandbox.DefaultMemory},
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

	for _, tc := range []struct {
		name string
		flag sandbox.Runtime
		rc   *config.AgenticRC
		want sandbox.Runtime
	}{
		{
			name: "flag takes precedence over rc",
			flag: sandbox.RuntimeDocker,
			rc:   rc,
			want: sandbox.RuntimeDocker,
		},
		{
			name: "rc fills an unset flag",
			rc:   rc,
			want: sandbox.RuntimeMicrosandbox,
		},
		{
			name: "defaults to docker",
			rc:   &config.AgenticRC{},
			want: sandbox.RuntimeDocker,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			runtime := Runtime(tc.flag, tc.rc)

			// Assert
			assert.Equal(t, tc.want, runtime)
		})
	}
}
