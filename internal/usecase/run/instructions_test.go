package run

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildInstructions(t *testing.T) {
	target := Target{ToolName: "claude", ImageName: "agentic-claude"}

	t.Run("disabled via config returns empty string", func(t *testing.T) {
		// Arrange
		disabled := false
		rc := &config.AgenticRC{Run: config.RCRun{Instructions: config.RCInstructions{Enabled: &disabled}}}

		// Act
		content, err := newTestService(&fakeDocker{}, nil).BuildInstructions(target, Input{}, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, content)
	})

	t.Run("image inspect error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) { return nil, fmt.Errorf("docker daemon unreachable") },
		}

		// Act
		_, err := newTestService(d, nil).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		assert.ErrorContains(t, err, "docker daemon unreachable")
	})

	built := &fakeDocker{
		inspectImage: func(string) (*docker.ImageInfo, error) {
			return &docker.ImageInfo{Base: "node@24.2.0,java@21.0.1", Apt: "make,gcc", CustomInstalls: "helm"}, nil
		},
		resolveContainerHome: func(string) string { return "/home/agent" },
	}
	noDaemon := &fakeMicrosandbox{capabilities: func() sandbox.Capabilities {
		return sandbox.Capabilities{Isolation: sandbox.Isolation{Boundary: sandbox.BoundaryVM}}
	}}
	custom := config.RCInstructions{Custom: "Always run go test before finishing."}
	proxyRC := config.RCProxy{
		AllowedHosts: []string{"extra.example.test"},
		AllowedPaths: []config.RCAllowedPath{
			{Host: "raw.example.test", Paths: []string{"/org/repo/**"}},
		},
	}

	cases := []struct {
		golden string
		docker *fakeDocker
		msb    *fakeMicrosandbox
		tool   string
		in     Input
		rc     config.RCRun
	}{
		{
			golden: "no-image",
			docker: &fakeDocker{},
			tool:   "claude",
		},
		{
			golden: "container",
			docker: built,
			tool:   "claude",
			in:     Input{Volumes: []string{"./data:/data:ro"}},
			rc:     config.RCRun{Mounts: []string{"go-mod:$CONTAINER_HOME/go"}, Instructions: custom},
		},
		{
			golden: "container-proxy-dind",
			docker: built,
			tool:   "claude",
			in:     Input{ProxyMode: sandbox.EgressEnforce, DindEnabled: true, DindLimits: sandbox.ResourceLimits{CPUs: "2"}},
			rc:     config.RCRun{Proxy: proxyRC, Dind: config.RCDind{RCLimits: config.RCLimits{Memory: "6g"}}},
		},
		{
			golden: "container-monitor",
			docker: built,
			tool:   "copilot",
			in:     Input{ProxyMode: sandbox.EgressMonitor},
			rc:     config.RCRun{Proxy: proxyRC},
		},
		{
			golden: "vm",
			docker: built,
			msb:    &fakeMicrosandbox{},
			tool:   "copilot",
			in:     Input{Runtime: sandbox.RuntimeMicrosandbox, Limits: sandbox.ResourceLimits{CPUs: "3", Memory: "6g"}},
			rc:     config.RCRun{Instructions: custom},
		},
		{
			golden: "vm-proxy-dind",
			docker: built,
			msb:    &fakeMicrosandbox{},
			tool:   "opencode",
			in:     Input{Runtime: sandbox.RuntimeMicrosandbox, ProxyMode: sandbox.EgressEnforce, DindEnabled: true},
			rc:     config.RCRun{Proxy: proxyRC},
		},
		{
			golden: "vm-agentic-proxy-dind",
			docker: built,
			msb:    &fakeMicrosandbox{},
			tool:   "claude",
			in:     Input{Runtime: sandbox.RuntimeMicrosandbox, ProxyBackend: config.BackendAgentic, ProxyMode: sandbox.EgressEnforce, DindEnabled: true},
		},
		{
			golden: "vm-no-daemon",
			docker: &fakeDocker{inspectImage: func(string) (*docker.ImageInfo, error) { return &docker.ImageInfo{}, nil }},
			msb:    noDaemon,
			tool:   "claude",
			in:     Input{Runtime: sandbox.RuntimeMicrosandbox, DindEnabled: true},
		},
	}
	for _, tc := range cases {
		t.Run("renders "+tc.golden, func(t *testing.T) {
			// Arrange
			var m Microsandbox
			if tc.msb != nil {
				m = tc.msb
			}

			// Act
			content, err := newTestService(tc.docker, m).BuildInstructions(target, tc.in, tools.Configs[tc.tool], &config.AgenticRC{Run: tc.rc})

			// Assert
			require.NoError(t, err)
			assertGolden(t, filepath.Join("instructions", tc.golden+".md"), content)
		})
	}
}

func TestPrepareInstructions(t *testing.T) {
	t.Run("disabled strips the persistent file's managed block without mounting anything", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		require.NoError(t, tools.Configs["claude"].Runtime.Setup(toolHome))
		require.NoError(t, tools.Configs["claude"].Runtime.WriteInstructions(toolHome, "stale block"))
		hostPath := tools.Configs["claude"].Runtime.InstructionsHostPath(toolHome)

		// Act
		snapshot, err := newTestService(&fakeDocker{}, nil).PrepareInstructions(toolHome, tools.Configs["claude"], "")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, snapshot.MountSpec)
		got, err := os.ReadFile(hostPath)
		require.NoError(t, err)
		assert.Empty(t, string(got))
	})

	t.Run("enabled mounts a per-run snapshot over the tool's instructions path", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		require.NoError(t, tools.Configs["claude"].Runtime.Setup(toolHome))

		// Act
		snapshot, err := newTestService(&fakeDocker{}, nil).PrepareInstructions(toolHome, tools.Configs["claude"], "generated block")

		// Assert
		require.NoError(t, err)
		t.Cleanup(snapshot.Cleanup)
		assert.Contains(t, snapshot.MountSpec, ":$CONTAINER_HOME/.claude/CLAUDE.md")
		hostPath := sandbox.HostPart(snapshot.MountSpec)
		got, err := os.ReadFile(hostPath)
		require.NoError(t, err)
		assert.Contains(t, string(got), "generated block")
	})

	// The exact merge/strip format is FinalizeInstructionsSnapshot's own
	// responsibility, already covered by TestFinalizeInstructionsSnapshot in the
	// tools package - this only checks that Cleanup's closure captured the
	// *correct* hostPath and snapshotPath for this tool/toolHome.
	t.Run("cleanup finalizes to this tool's persistent path and removes the snapshot", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		require.NoError(t, tools.Configs["claude"].Runtime.Setup(toolHome))
		snapshot, err := newTestService(&fakeDocker{}, nil).PrepareInstructions(toolHome, tools.Configs["claude"], "generated block")
		require.NoError(t, err)
		snapshotPath := sandbox.HostPart(snapshot.MountSpec)
		require.NoError(t, appendFile(snapshotPath, "\nuser note\n"))

		// Act
		snapshot.Cleanup()

		// Assert
		_, statErr := os.Stat(snapshotPath)
		assert.True(t, os.IsNotExist(statErr))
		hostPath := tools.Configs["claude"].Runtime.InstructionsHostPath(toolHome)
		got, err := os.ReadFile(hostPath)
		require.NoError(t, err)
		assert.Contains(t, string(got), "user note")
	})
}

func TestPreviewInstructions(t *testing.T) {
	t.Run("merges the persisted host file's content into the preview", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		require.NoError(t, req.Tool.Runtime.Setup(req.Flags.ToolHome))
		hostPath := req.Tool.Runtime.InstructionsHostPath(req.Flags.ToolHome)
		require.NoError(t, os.WriteFile(hostPath, []byte("my own global notes\n"), 0o640))

		// Act
		content, err := newTestService(&fakeDocker{}, nil).PreviewInstructions(req)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "## Precedence")
		assert.Contains(t, content, "my own global notes")
	})

	t.Run("disabled via config returns empty string", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		disabled := false
		req.Project.RC.Run.Instructions.Enabled = &disabled

		// Act
		content, err := newTestService(&fakeDocker{}, nil).PreviewInstructions(req)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, content)
	})

	t.Run("proxy flags are resolved like a run", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Flags.Proxy = resolve.ProxyInput{ProxyFlag: true}

		// Act
		content, err := newTestService(&fakeDocker{}, nil).PreviewInstructions(req)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "## Network")
	})

	t.Run("a proxy conflict errors like a run", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Project.RC.Run.Proxy.Credentials = []config.RCCredential{{Hosts: []string{"api.example.test"}, Header: "X-Api-Key", Secret: "/example.test/key"}}
		req.Flags.Proxy = resolve.ProxyInput{NoProxy: true}

		// Act
		_, err := newTestService(&fakeDocker{}, nil).PreviewInstructions(req)

		// Assert
		require.ErrorContains(t, err, "--no-proxy cannot be used")
	})
}

func Test_instructionsEnabled(t *testing.T) {
	t.Run("unset defaults to enabled", func(t *testing.T) {
		// Act
		enabled := instructionsEnabled(&config.AgenticRC{})

		// Assert
		assert.True(t, enabled)
	})

	t.Run("explicit true is enabled", func(t *testing.T) {
		// Arrange
		on := true
		rc := &config.AgenticRC{Run: config.RCRun{Instructions: config.RCInstructions{Enabled: &on}}}

		// Act
		enabled := instructionsEnabled(rc)

		// Assert
		assert.True(t, enabled)
	})

	t.Run("explicit false is disabled", func(t *testing.T) {
		// Arrange
		off := false
		rc := &config.AgenticRC{Run: config.RCRun{Instructions: config.RCInstructions{Enabled: &off}}}

		// Act
		enabled := instructionsEnabled(rc)

		// Assert
		assert.False(t, enabled)
	})
}
