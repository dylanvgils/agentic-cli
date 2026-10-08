package run

import (
	"fmt"
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/tools"
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
		content, err := New(&fakeDocker{}).BuildInstructions(target, Input{}, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, content)
	})

	t.Run("enabled by default when unset", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.NotEmpty(t, content)
	})

	t.Run("precedence section defers to the project's own instructions file", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "## Precedence")
		assert.Contains(t, content, "CLAUDE.md, AGENTS.md, copilot-instructions.md")
	})

	t.Run("image not found notes capabilities are unavailable", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "remaining capability details unavailable")
		assert.Contains(t, content, "curl")
	})

	t.Run("image inspect error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) { return nil, fmt.Errorf("docker daemon unreachable") },
		}

		// Act
		_, err := New(d).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		assert.ErrorContains(t, err, "docker daemon unreachable")
	})

	t.Run("capabilities reflect image labels", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) {
				return &docker.ImageInfo{Base: "node@24.2.0,java@21.0.1", Apt: "make,gcc", CustomInstalls: "helm"}, nil
			},
		}

		// Act
		content, err := New(d).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "  - node@24.2.0\n")
		assert.Contains(t, content, "  - java@21.0.1\n")
		assert.Contains(t, content, "  - make\n")
		assert.Contains(t, content, "  - gcc\n")
		assert.Contains(t, content, "  - helm\n")
		assert.Contains(t, content, "curl")
	})

	t.Run("no extras notes base debian only", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) { return &docker.ImageInfo{}, nil },
		}

		// Act
		content, err := New(d).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "none (base Debian image only)")
		assert.Contains(t, content, "curl")
	})

	t.Run("notes missing tools must be installed by the user", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "cannot be installed or run")
		assert.Contains(t, content, "custom_installs in .agenticrc.toml")
	})

	t.Run("base toolchain always listed", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "- Base toolchain (always installed):\n")
		for _, pkg := range tools.BasePackages() {
			assert.Contains(t, content, "  - "+pkg+"\n")
		}
	})

	t.Run("filesystem section lists tmpfs paths", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}).BuildInstructions(target, Input{}, tools.Configs["copilot"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "root filesystem is read-only")
		assert.Contains(t, content, "/workspace")
		assert.Contains(t, content, "/tmp")
		assert.Contains(t, content, ".cache")
	})

	t.Run("resource limits reflect resolved defaults", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, docker.DefaultPidsLimit)
		assert.Contains(t, content, docker.DefaultCPUs)
		assert.Contains(t, content, docker.DefaultMemory)
	})

	t.Run("resource limits note they are configurable", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "configurable, not hard caps")
		assert.Contains(t, content, "pids_limit/cpus/memory in .agenticrc.toml")
	})

	t.Run("network section omitted when proxy disabled", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, content, "## Network")
	})

	t.Run("network section lists allowlist when enforcing", func(t *testing.T) {
		// Arrange
		in := Input{ProxyMode: docker.ProxyEnforce}
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{AllowedHosts: []string{"extra.example.com"}}}}

		// Act
		content, err := New(&fakeDocker{}).BuildInstructions(target, in, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "no direct internet access")
		assert.Contains(t, content, "extra.example.com")
		assert.Contains(t, content, ".anthropic.com")
		assert.Contains(t, content, "tell the user why so they can add it to allowed_hosts")
	})

	t.Run("network section omits host list in monitor mode", func(t *testing.T) {
		// Arrange
		in := Input{ProxyMode: docker.ProxyMonitor}
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{AllowedHosts: []string{"extra.example.com"}}}}

		// Act
		content, err := New(&fakeDocker{}).BuildInstructions(target, in, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "no direct internet access")
		assert.NotContains(t, content, "extra.example.com")
		assert.NotContains(t, content, "reachable")
		assert.NotContains(t, content, "tell the user why so they can add it to allowed_hosts")
	})

	t.Run("docker section omitted when dind disabled", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, content, "## Docker")
	})

	t.Run("docker section explains the sidecar when dind enabled", func(t *testing.T) {
		// Arrange
		in := Input{DindEnabled: true}

		// Act
		content, err := New(&fakeDocker{}).BuildInstructions(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "## Docker")
		assert.Contains(t, content, "agentic-docker:<port>")
		assert.NotContains(t, content, "egress proxy")
	})

	t.Run("docker section lists the sidecar limits", func(t *testing.T) {
		// Arrange
		in := Input{DindEnabled: true, DindLimits: docker.ResourceLimits{CPUs: "2"}}
		rc := &config.AgenticRC{Run: config.RCRun{Dind: config.RCDind{RCLimits: config.RCLimits{Memory: "8g"}}}}

		// Act
		content, err := New(&fakeDocker{}).BuildInstructions(target, in, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "own limits, shared by everything it runs: "+docker.DefaultPidsLimit+" processes, 2 CPUs, 8g memory")
	})

	t.Run("docker section and allowlist cover registries with proxy", func(t *testing.T) {
		// Arrange
		in := Input{DindEnabled: true, ProxyMode: docker.ProxyEnforce}

		// Act
		content, err := New(&fakeDocker{}).BuildInstructions(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "Image pulls, builds and containers go through the same egress proxy")
		assert.Contains(t, content, "registry-1.docker.io")
	})

	t.Run("custom instructions appended when set", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Instructions: config.RCInstructions{Custom: "Always run go test before finishing."}}}

		// Act
		content, err := New(&fakeDocker{}).BuildInstructions(target, Input{}, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "## Additional instructions")
		assert.Contains(t, content, "Always run go test before finishing.")
	})

	t.Run("custom section omitted when unset", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, content, "## Additional instructions")
	})
}

func TestPrepareInstructions(t *testing.T) {
	t.Run("disabled strips the persistent file's managed block without mounting anything", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		require.NoError(t, tools.Configs["claude"].Runtime.Setup(toolHome))
		require.NoError(t, tools.Configs["claude"].Runtime.WriteInstructions(toolHome, "stale block"))
		hostPath := tools.Configs["claude"].Runtime.InstructionsHostPath(toolHome)

		// Act
		snapshot, err := New(&fakeDocker{}).PrepareInstructions(toolHome, tools.Configs["claude"], "")

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
		snapshot, err := New(&fakeDocker{}).PrepareInstructions(toolHome, tools.Configs["claude"], "generated block")

		// Assert
		require.NoError(t, err)
		t.Cleanup(snapshot.Cleanup)
		assert.Contains(t, snapshot.MountSpec, ":$CONTAINER_HOME/.claude/CLAUDE.md")
		hostPath := mount.HostPart(snapshot.MountSpec)
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
		snapshot, err := New(&fakeDocker{}).PrepareInstructions(toolHome, tools.Configs["claude"], "generated block")
		require.NoError(t, err)
		snapshotPath := mount.HostPart(snapshot.MountSpec)
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
		content, err := New(&fakeDocker{}).PreviewInstructions(req)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "## Precedence")
		assert.Contains(t, content, "my own global notes")
	})

	t.Run("host file that does not exist yet is not an error", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)

		// Act
		content, err := New(&fakeDocker{}).PreviewInstructions(req)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "## Precedence")
	})

	t.Run("disabled via config returns empty string without reading the host file", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		disabled := false
		req.Project.RC.Run.Instructions.Enabled = &disabled

		// Act
		content, err := New(&fakeDocker{}).PreviewInstructions(req)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, content)
	})

	t.Run("proxy flags are resolved like a run", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Flags.Proxy = resolve.ProxyInput{ProxyFlag: true}

		// Act
		content, err := New(&fakeDocker{}).PreviewInstructions(req)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "## Network")
	})

	t.Run("a proxy conflict errors like a run", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Project.RC.Run.Proxy.Credentials = credentialLayer(t, "/example.test/key").RC.Run.Proxy.Credentials
		req.Flags.Proxy = resolve.ProxyInput{NoProxy: true}

		// Act
		_, err := New(&fakeDocker{}).PreviewInstructions(req)

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
