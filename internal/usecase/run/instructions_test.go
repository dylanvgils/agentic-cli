package run

import (
	"fmt"
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
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
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, Input{}, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, content)
	})

	t.Run("precedence section defers to the project's own instructions file", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "## Precedence")
		assert.Contains(t, content, "CLAUDE.md, AGENTS.md, copilot-instructions.md")
	})

	t.Run("image not found notes capabilities are unavailable", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "remaining capability details unavailable")
	})

	t.Run("image inspect error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) { return nil, fmt.Errorf("docker daemon unreachable") },
		}

		// Act
		_, err := New(d, nil).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

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
		content, err := New(d, nil).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "  - node@24.2.0\n")
		assert.Contains(t, content, "  - java@21.0.1\n")
		assert.Contains(t, content, "  - make\n")
		assert.Contains(t, content, "  - gcc\n")
		assert.Contains(t, content, "  - helm\n")
	})

	t.Run("no extras notes base debian only", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) { return &docker.ImageInfo{}, nil },
		}

		// Act
		content, err := New(d, nil).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "none (base Debian image only)")
	})

	t.Run("notes missing tools must be installed by the user", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "Package managers work only when they install into a writable path")
		assert.Contains(t, content, "custom_installs in .agenticrc.toml")
	})

	t.Run("base toolchain always listed", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "- Base toolchain (always installed):\n")
		for _, pkg := range tools.BasePackages() {
			assert.Contains(t, content, "  - "+pkg+"\n")
		}
	})

	t.Run("filesystem section lists the ephemeral dirs", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{resolveContainerHome: func(string) string { return "/home/agent" }}

		// Act
		content, err := New(d, nil).BuildInstructions(target, Input{}, tools.Configs["copilot"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "- `/workspace`: the project on the user's machine")
		assert.Contains(t, content, "- `/tmp`: writable, cleared when the container exits.\n")
		assert.Contains(t, content, "- `/home/agent/.cache`: writable, cleared when the container exits.\n")
		assert.Contains(t, content, "- Every other path is read-only.\n")
	})

	t.Run("filesystem section lists mounts with their access", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{resolveContainerHome: func(string) string { return "/home/agent" }}
		in := Input{Volumes: []string{"./data:/data"}}
		rc := &config.AgenticRC{Run: config.RCRun{Mounts: []string{"go-mod:$CONTAINER_HOME/go"}}}

		// Act
		content, err := New(d, nil).BuildInstructions(target, in, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "- `/home/agent/go`: writable, persists across runs.\n")
		assert.Contains(t, content, "- `/data`: read-only, persists across runs.\n")
		assert.NotContains(t, content, "- `/workspace`: writable")
	})

	t.Run("resource limits reflect resolved defaults", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, sandbox.DefaultPidsLimit)
		assert.Contains(t, content, sandbox.DefaultCPUs)
		assert.Contains(t, content, sandbox.DefaultMemory)
	})

	t.Run("resource limits note they are configurable", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "hard limits for this run")
		assert.Contains(t, content, "pids_limit/cpus/memory in .agenticrc.toml")
	})

	t.Run("network section omitted when proxy disabled", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, content, "## Network")
	})

	t.Run("network section lists allowlist when enforcing", func(t *testing.T) {
		// Arrange
		in := Input{ProxyMode: sandbox.ProxyEnforce}
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{AllowedHosts: []string{"extra.example.com"}}}}

		// Act
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, in, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "- All traffic goes through an egress proxy.\n")
		assert.Contains(t, content, "only on ports 80 and 443")
		assert.Contains(t, content, "  - .anthropic.com\n")
		assert.Contains(t, content, "extra.example.com")
		assert.Contains(t, content, "HTTP 403 from the proxy")
		assert.Contains(t, content, "so they can add it to allowed_hosts")
	})

	t.Run("network section names a provider-side web search", func(t *testing.T) {
		// Arrange
		in := Input{ProxyMode: sandbox.ProxyEnforce}

		// Act
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "Web search (`WebSearch`) runs on the model provider's side")
		assert.Contains(t, content, "Don't retry it another way")
	})

	t.Run("network section limits searches to the allowlist without a provider-side search", func(t *testing.T) {
		// Arrange
		in := Input{ProxyMode: sandbox.ProxyEnforce}

		// Act
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, in, tools.Configs["opencode"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "Web searches and page fetches (any search or fetch tool, `curl`, `wget`) only reach the hosts above.")
		assert.NotContains(t, content, "model provider's side")
	})

	t.Run("network section omits host list in monitor mode", func(t *testing.T) {
		// Arrange
		in := Input{ProxyMode: sandbox.ProxyMonitor}
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{AllowedHosts: []string{"extra.example.com"}}}}

		// Act
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, in, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "logs every request and blocks nothing")
		assert.NotContains(t, content, "extra.example.com")
		assert.NotContains(t, content, "Only these hosts")
		assert.NotContains(t, content, "allowed_hosts")
	})

	t.Run("docker section omitted when dind disabled", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, content, "## Docker")
	})

	t.Run("docker section explains the sidecar when dind enabled", func(t *testing.T) {
		// Arrange
		in := Input{DindEnabled: true}

		// Act
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "## Docker")
		assert.Contains(t, content, "agentic-docker:<port>")
		assert.NotContains(t, content, "egress proxy")
	})

	t.Run("docker section lists the sidecar limits", func(t *testing.T) {
		// Arrange
		in := Input{DindEnabled: true, DindLimits: sandbox.ResourceLimits{CPUs: "2"}}
		rc := &config.AgenticRC{}
		rc.Run.Dind.Memory = "6g"

		// Act
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, in, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, fmt.Sprintf("%s processes, 2 CPUs, 6g memory", sandbox.DefaultPidsLimit))
	})

	t.Run("docker section and allowlist cover registries with proxy", func(t *testing.T) {
		// Arrange
		in := Input{DindEnabled: true, ProxyMode: sandbox.ProxyEnforce}

		// Act
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "Image pulls, builds and containers use the same allowlist")
		assert.Contains(t, content, "  - registry-1.docker.io\n")
	})

	t.Run("microsandbox describes a VM instead of the hardened container", func(t *testing.T) {
		// Arrange
		in := Input{Runtime: sandbox.RuntimeMicrosandbox, Limits: sandbox.ResourceLimits{CPUs: "3", Memory: "6g"}}

		// Act
		content, err := New(&fakeDocker{}, &fakeMicrosandbox{}).BuildInstructions(target, in, tools.Configs["copilot"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "inside an agentic-cli microVM")
		assert.Contains(t, content, "## Precedence")
		assert.Contains(t, content, "- Base toolchain (always installed):\n")
		assert.Contains(t, content, "- Non-root user. `sudo` and apt fail.\n")
		assert.Contains(t, content, "`/tmp` included, is writable but discarded when the VM exits")
		assert.NotContains(t, content, ".cache")
		assert.Contains(t, content, "- CPUs: 3\n- Memory: 6g\n")
		assert.Contains(t, content, "Secrets under `/run/secrets/` are read-only")
		assert.NotContains(t, content, "root filesystem is read-only")
		assert.NotContains(t, content, "pids")
		assert.NotContains(t, content, "capabilities dropped")
	})

	t.Run("microsandbox describes its network policy instead of the proxy", func(t *testing.T) {
		// Arrange
		in := Input{Runtime: sandbox.RuntimeMicrosandbox, ProxyMode: sandbox.ProxyEnforce}

		// Act
		content, err := New(&fakeDocker{}, &fakeMicrosandbox{}).BuildInstructions(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "The VM's network policy filters all traffic and inspects HTTPS")
		assert.Contains(t, content, "fails with a DNS, connection or TLS error")
		assert.Contains(t, content, "  - .anthropic.com\n")
		assert.NotContains(t, content, "egress proxy")
	})

	t.Run("microsandbox omits the sidecar it lacks even when dind is enabled", func(t *testing.T) {
		// Arrange
		in := Input{Runtime: sandbox.RuntimeMicrosandbox, DindEnabled: true}

		// Act
		content, err := New(&fakeDocker{}, &fakeMicrosandbox{}).BuildInstructions(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, content, "sidecar")
		assert.Contains(t, content, "no Docker daemon in this VM")
	})

	t.Run("microsandbox omits the docker section when dind disabled", func(t *testing.T) {
		// Arrange
		in := Input{Runtime: sandbox.RuntimeMicrosandbox}

		// Act
		content, err := New(&fakeDocker{}, &fakeMicrosandbox{}).BuildInstructions(target, in, tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, content, "## Docker")
	})

	t.Run("microsandbox still appends custom instructions", func(t *testing.T) {
		// Arrange
		in := Input{Runtime: sandbox.RuntimeMicrosandbox}
		rc := &config.AgenticRC{Run: config.RCRun{Instructions: config.RCInstructions{Custom: "Always run go test before finishing."}}}

		// Act
		content, err := New(&fakeDocker{}, &fakeMicrosandbox{}).BuildInstructions(target, in, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "Always run go test before finishing.")
	})

	t.Run("custom instructions appended when set", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Instructions: config.RCInstructions{Custom: "Always run go test before finishing."}}}

		// Act
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, Input{}, tools.Configs["claude"], rc)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "## Additional instructions")
		assert.Contains(t, content, "Always run go test before finishing.")
	})

	t.Run("custom section omitted when unset", func(t *testing.T) {
		// Act
		content, err := New(&fakeDocker{}, nil).BuildInstructions(target, Input{}, tools.Configs["claude"], &config.AgenticRC{})

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
		snapshot, err := New(&fakeDocker{}, nil).PrepareInstructions(toolHome, tools.Configs["claude"], "")

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
		snapshot, err := New(&fakeDocker{}, nil).PrepareInstructions(toolHome, tools.Configs["claude"], "generated block")

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
		snapshot, err := New(&fakeDocker{}, nil).PrepareInstructions(toolHome, tools.Configs["claude"], "generated block")
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
		content, err := New(&fakeDocker{}, nil).PreviewInstructions(req)

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
		content, err := New(&fakeDocker{}, nil).PreviewInstructions(req)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, content)
	})

	t.Run("proxy flags are resolved like a run", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Flags.Proxy = resolve.ProxyInput{ProxyFlag: true}

		// Act
		content, err := New(&fakeDocker{}, nil).PreviewInstructions(req)

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
		_, err := New(&fakeDocker{}, nil).PreviewInstructions(req)

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
