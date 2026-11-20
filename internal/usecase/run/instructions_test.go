package run

import (
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

func Test_Service_buildInstructions(t *testing.T) {
	target := Target{ToolName: "claude", ImageName: "agentic-claude"}

	t.Run("disabled via config returns empty string", func(t *testing.T) {
		// Arrange
		disabled := false
		c := newTestRunConfig(t)
		c.RC = &config.AgenticRC{Run: config.RCRun{Instructions: config.RCInstructions{Enabled: &disabled}}}

		// Act
		content, err := newTestService(&fakeDocker{}, nil).buildInstructions(c)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, content)
	})

	built := &fakeDocker{
		inspectImage: func(string) (*docker.ImageInfo, error) {
			return &docker.ImageInfo{
				Base:           "node@24.2.0,java@21.0.1",
				Apt:            "make,gcc",
				CustomInstalls: "helm",
				ContainerHome:  "/home/agent",
			}, nil
		},
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
		flags  Flags
		rc     config.RCRun
		// pin sets what resolveRun wouldn't, to reach a renderer branch
		pin func(c *runConfig)
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
			flags:  Flags{Volumes: []string{"./data:/data:ro"}},
			rc:     config.RCRun{Mounts: []string{"go-mod:$CONTAINER_HOME/go"}, Instructions: custom},
		},
		{
			golden: "container-proxy-dind",
			docker: built,
			tool:   "claude",
			flags: Flags{
				Proxy:      resolve.ProxyInput{ProxyFlag: true},
				Dind:       resolve.DindInput{DindFlag: true},
				DindLimits: sandbox.ResourceLimits{CPUs: "2"},
			},
			rc: config.RCRun{
				Proxy: proxyRC,
				Dind:  config.RCDind{RCLimits: config.RCLimits{Memory: "6g"}},
			},
		},
		{
			golden: "container-ports-dind",
			docker: built,
			tool:   "claude",
			flags: Flags{
				Proxy: resolve.ProxyInput{NoProxy: true},
				Dind:  resolve.DindInput{DindFlag: true},
				Ports: []string{
					"3000",
					"8080:5173",
				},
			},
		},
		{
			golden: "container-monitor",
			docker: built,
			tool:   "copilot",
			flags:  Flags{Proxy: resolve.ProxyInput{MonitorFlag: true}},
			rc:     config.RCRun{Proxy: proxyRC},
		},
		{
			golden: "vm",
			docker: built,
			msb:    &fakeMicrosandbox{},
			tool:   "copilot",
			flags: Flags{
				Runtime: sandbox.RuntimeMicrosandbox,
				Limits: sandbox.ResourceLimits{
					CPUs:   "3",
					Memory: "6g",
				},
			},
			rc: config.RCRun{Instructions: custom},
		},
		{
			golden: "vm-proxy-dind",
			docker: built,
			msb:    &fakeMicrosandbox{},
			tool:   "opencode",
			flags: Flags{
				Runtime: sandbox.RuntimeMicrosandbox,
				Proxy:   resolve.ProxyInput{ProxyFlag: true},
				Dind:    resolve.DindInput{DindFlag: true},
			},
			rc: config.RCRun{Proxy: proxyRC},
		},
		{
			golden: "vm-ports-dind",
			docker: built,
			msb:    &fakeMicrosandbox{},
			tool:   "claude",
			flags: Flags{
				Runtime: sandbox.RuntimeMicrosandbox,
				Dind:    resolve.DindInput{DindFlag: true},
				Ports:   []string{"3000"},
			},
		},
		{
			golden: "vm-agentic-proxy-dind",
			docker: built,
			msb:    &fakeMicrosandbox{},
			tool:   "claude",
			flags: Flags{
				Runtime: sandbox.RuntimeMicrosandbox,
				Proxy: resolve.ProxyInput{
					ProxyFlag: true,
					Backend:   config.BackendAgentic,
				},
				Dind: resolve.DindInput{DindFlag: true},
			},
		},
		{
			golden: "vm-no-daemon",
			docker: &fakeDocker{inspectImage: func(string) (*docker.ImageInfo, error) { return &docker.ImageInfo{}, nil }},
			msb:    noDaemon,
			tool:   "claude",
			flags:  Flags{Runtime: sandbox.RuntimeMicrosandbox},
			// resolveRun keeps dind off without a daemon, so no run reaches this fallback
			pin: func(c *runConfig) { c.Dind.Enabled = true },
		},
	}
	for _, tc := range cases {
		t.Run("renders "+tc.golden, func(t *testing.T) {
			// Arrange
			var m Microsandbox
			if tc.msb != nil {
				m = tc.msb
			}
			svc := newTestService(tc.docker, m)
			req := Request{
				Target: target,
				Tool:   tools.Configs[tc.tool],
				Flags:  tc.flags,
			}
			c, err := svc.resolveRun(req, &config.AgenticRC{Run: tc.rc})
			require.NoError(t, err)
			if tc.pin != nil {
				tc.pin(&c)
			}

			// Act
			content, err := svc.buildInstructions(c)

			// Assert
			require.NoError(t, err)
			assertGolden(t, filepath.Join("instructions", tc.golden+".md"), content)
		})
	}
}

func Test_Service_prepareInstructions(t *testing.T) {
	t.Run("disabled strips the persistent file's managed block without mounting anything", func(t *testing.T) {
		// Arrange
		c := newTestRunConfig(t)
		require.NoError(t, c.Tool.Runtime.Setup(c.Homes.Tool))
		require.NoError(t, c.Tool.Runtime.WriteInstructions(c.Homes.Tool, "stale block"))
		hostPath := c.Tool.Runtime.InstructionsHostPath(c.Homes.Tool)

		// Act
		mountSpec, err := newTestService(&fakeDocker{}, nil).prepareInstructions(c, testRunID, "")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, mountSpec)
		got, err := os.ReadFile(hostPath)
		require.NoError(t, err)
		assert.Empty(t, string(got))
	})

	t.Run("enabled mounts the run's snapshot over the tool's instructions path", func(t *testing.T) {
		// Arrange
		stubRunTempDir(t)
		c := newTestRunConfig(t)
		require.NoError(t, c.Tool.Runtime.Setup(c.Homes.Tool))

		// Act
		mountSpec, err := newTestService(&fakeDocker{}, nil).prepareInstructions(c, testRunID, "generated block")

		// Assert
		require.NoError(t, err)
		snapshot := filepath.Join(sandbox.RunTempDir(testRunID), "instructions.md")
		assert.Equal(t, sandbox.VolumeMount(snapshot, "$CONTAINER_HOME/.claude/CLAUDE.md"), mountSpec)
		got, err := os.ReadFile(snapshot)
		require.NoError(t, err)
		assert.Contains(t, string(got), "generated block")
	})
}

// The exact merge/strip format is FinalizeSnapshot's own, covered in the instructions package; this only checks that
// saveSnapshot finds the run's snapshot and its tool's file.
func Test_saveSnapshot(t *testing.T) {
	t.Run("saves the run's snapshot to its tool's file and removes it", func(t *testing.T) {
		// Arrange
		stubRunTempDir(t)
		run := sandbox.RunRef{ID: testRunID, Tool: "claude", ToolHome: t.TempDir()}
		require.NoError(t, tools.Configs["claude"].Runtime.Setup(run.ToolHome))
		c := newTestRunConfig(t)
		c.Homes.Tool = run.ToolHome
		mountSpec, err := newTestService(&fakeDocker{}, nil).prepareInstructions(c, run.ID, "generated block")
		require.NoError(t, err)
		snapshot := sandbox.HostPart(mountSpec)
		require.NoError(t, appendFile(snapshot, "\nuser note\n"))

		// Act
		err = saveSnapshot(run)

		// Assert
		require.NoError(t, err)
		assert.NoFileExists(t, snapshot)
		got, err := os.ReadFile(tools.Configs["claude"].Runtime.InstructionsHostPath(run.ToolHome))
		require.NoError(t, err)
		assert.Equal(t, "user note\n", string(got))
	})

	for _, tc := range []struct {
		name string
		tool string
	}{
		{
			name: "without a snapshot there is nothing to save",
			tool: "claude",
		},
		{
			name: "an unknown tool has nothing to save",
			tool: "example",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			stubRunTempDir(t)
			run := sandbox.RunRef{ID: testRunID, Tool: tc.tool, ToolHome: t.TempDir()}

			// Act
			err := saveSnapshot(run)

			// Assert
			require.NoError(t, err)
		})
	}
}

func TestPreviewInstructions(t *testing.T) {
	t.Run("merges the persisted host file's content into the preview", func(t *testing.T) {
		// Arrange
		req := newResolveRequest(t)
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
		req := newResolveRequest(t)
		disabled := false
		req.Project.RC.Run.Instructions.Enabled = &disabled

		// Act
		content, err := newTestService(&fakeDocker{}, nil).PreviewInstructions(req)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, content)
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
