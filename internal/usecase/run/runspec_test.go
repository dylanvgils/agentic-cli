package run

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/credentials"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Service_build(t *testing.T) {
	trusted := func(string) (*docker.ImageInfo, error) { return &docker.ImageInfo{ProxyTrust: true}, nil }
	d := &fakeDocker{inspectImage: trusted}

	t.Run("volumes wired", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), Volumes: []string{"/host:/container:rw"}}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		assert.Contains(t, rs.Volumes, "/host:/container:rw")
	})

	t.Run("runs as the host user, with a TTY only when stdin is a terminal", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, platform.HostUser(), *rs.User)
		assert.Equal(t, platform.IsTerminal(), rs.TTY)
	})

	t.Run("instructions mount wired after the tool's base mounts", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), InstructionsMount: "/tmp/snapshot.md:$CONTAINER_HOME/.claude/CLAUDE.md"}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		require.Contains(t, rs.Volumes, "/tmp/snapshot.md:$CONTAINER_HOME/.claude/CLAUDE.md")
		baseMountIdx := slices.Index(rs.Volumes, tools.Configs["claude"].Runtime.Mounts()[1])
		instructionsMountIdx := slices.Index(rs.Volumes, "/tmp/snapshot.md:$CONTAINER_HOME/.claude/CLAUDE.md")
		assert.Less(t, baseMountIdx, instructionsMountIdx, "instructions mount must overlay the base directory mount, so it must be listed after it")
	})

	t.Run("instructions mount omitted when empty", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		for _, v := range rs.Volumes {
			assert.NotContains(t, v, "CLAUDE.md")
		}
	})

	t.Run("secrets wired", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), Secrets: []string{"mytoken:/tmp/token"}}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"mytoken:/tmp/token"}, rs.Secrets)
	})

	t.Run("resource limits wired", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}
		rc := &config.AgenticRC{Run: config.RCRun{RCLimits: config.RCLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}}}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(rc))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}, rs.Limits)
	})

	t.Run("dind limits inherit tool limits", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}
		rc := &config.AgenticRC{Run: config.RCRun{RCLimits: config.RCLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}}}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(rc))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}, rs.Dind.Limits)
	})

	t.Run("dind limits override tool limits", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), DindLimits: sandbox.ResourceLimits{Memory: "16g"}}
		rc := &config.AgenticRC{Run: config.RCRun{
			RCLimits: config.RCLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"},
			Dind:     config.RCDind{RCLimits: config.RCLimits{PidsLimit: "4096", Memory: "8g"}},
		}}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(rc))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ResourceLimits{PidsLimit: "4096", CPUs: "2", Memory: "16g"}, rs.Dind.Limits)
		assert.Equal(t, "2g", rs.Limits.Memory, "tool limit is unaffected")
	})

	t.Run("dry run wired", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), DryRun: true}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		assert.True(t, rs.DryRun)
	})

	t.Run("ephemeral dirs wired", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		assert.NotEmpty(t, rs.EphemeralDirs)
	})

	t.Run("tool home wired", func(t *testing.T) {
		// Arrange
		customHome := t.TempDir()
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: customHome}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, customHome, rs.ToolHome)
	})

	t.Run("skip entrypoint wired", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude", SkipEntrypoint: true}
		in := Input{ToolHome: t.TempDir()}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		assert.True(t, rs.SkipEntrypoint)
	})

	t.Run("config mount through a planted symlink out of cwd is refused", func(t *testing.T) {
		// Arrange
		chdirSymlinkWorkspace(t)
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}
		rc := &config.AgenticRC{Run: config.RCRun{Mounts: []string{"$PWD/escape:/workspace/escape"}}}

		// Act
		_, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(rc))

		// Assert
		assert.ErrorContains(t, err, "goes through workspace symlink")
	})

	t.Run("proxy wired with merged allowlist and image", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), ProxyMode: sandbox.ProxyEnforce}
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{AllowedHosts: []string{"extra.example.com"}}}}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(rc))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ProxyEnforce, rs.Proxy.Mode)
		assert.Equal(t, []string{".anthropic.com", ".claude.ai", ".claude.com", "extra.example.com"}, rs.Proxy.Hosts())
		assert.NotEmpty(t, rs.Proxy.LogFile)
	})

	t.Run("proxy off leaves spec unproxied", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ProxyOff, rs.Proxy.Mode)
		assert.Empty(t, rs.Proxy.LogFile)
	})

	t.Run("proxy monitor wired", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), ProxyMode: sandbox.ProxyMonitor}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ProxyMonitor, rs.Proxy.Mode)
	})

	t.Run("proxy credentials wired for the docker runtime to place", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		resolved := credentials.Resolved{Hosts: []string{"api.example.test"}, Headers: []string{"X-Api-Key"}, Env: []string{"EXAMPLE_API_KEY"}, Secret: "test-secret"}
		in := Input{
			ToolHome:    t.TempDir(),
			ProxyMode:   sandbox.ProxyEnforce,
			Credentials: []credentials.Resolved{resolved},
		}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []sandbox.Credential{
			{Name: "EXAMPLE_API_KEY", Aliases: []string{}, Hosts: []string{"api.example.test"}, Headers: []string{"X-Api-Key"}, Secret: "test-secret"},
		}, rs.Credentials)
		assert.Equal(t, sandbox.ProxyEnforce, rs.Proxy.Mode)
	})

	t.Run("microsandbox credentials wired without the proxy, placeholder env or image inspect", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{}
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		resolved := credentials.Resolved{Hosts: []string{"api.example.test"}, Headers: []string{"X-Api-Key"}, Env: []string{"EXAMPLE_API_KEY"}, Secret: "test-secret"}
		in := Input{
			ToolHome:    t.TempDir(),
			Runtime:     sandbox.RuntimeMicrosandbox,
			Credentials: []credentials.Resolved{resolved},
		}

		// Act
		rs, err := New(d, &fakeMicrosandbox{}).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []sandbox.Credential{
			{Name: "EXAMPLE_API_KEY", Aliases: []string{}, Hosts: []string{"api.example.test"}, Headers: []string{"X-Api-Key"}, Secret: "test-secret"},
		}, rs.Credentials)
	})

	t.Run("proxy credentials refused on an image without proxy trust", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) { return &docker.ImageInfo{}, nil },
		}
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		resolved := credentials.Resolved{Hosts: []string{"api.example.test"}, Env: []string{"EXAMPLE_API_KEY"}}
		in := Input{ToolHome: t.TempDir(), ProxyMode: sandbox.ProxyEnforce, Credentials: []credentials.Resolved{resolved}}

		// Act
		_, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		assert.ErrorContains(t, err, "agentic update claude")
	})

	t.Run("proxy credentials rejected when the secret is mounted", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		dir := t.TempDir()
		in := Input{
			ToolHome:    t.TempDir(),
			Volumes:     []string{dir + ":/data"},
			ProxyMode:   sandbox.ProxyEnforce,
			Credentials: []credentials.Resolved{{Path: filepath.Join(dir, "token")}},
		}

		// Act
		_, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		assert.ErrorContains(t, err, "which the tool container can access")
	})

	t.Run("dind wired with sidecar image and docker hub allowlisted", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: trusted}
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), ProxyMode: sandbox.ProxyEnforce, DindEnabled: true}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert - agentic-net is skipped since the tool joins a per-run network
		require.NoError(t, err)
		assert.True(t, rs.Dind.Enabled)
		assert.Contains(t, rs.Proxy.Hosts(), "registry-1.docker.io")
	})

	t.Run("dind rejects env overriding the docker host", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), DindEnabled: true, Env: []string{"DOCKER_HOST=unix:///var/run/docker.sock"}}

		// Act
		_, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		assert.ErrorContains(t, err, "DOCKER_HOST")
	})

	t.Run("marketplace names wired into AGENTIC_MARKETPLACES env", func(t *testing.T) {
		// Arrange
		svc := newSyncingService(d, func(entries []marketplace.Entry, dirFor func(marketplace.Entry) string) ([]marketplace.Result, error) {
			return []marketplace.Result{{Entry: entries[0], Dir: dirFor(entries[0])}}, nil
		})
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}
		rc := &config.AgenticRC{Marketplaces: []config.RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}

		// Act
		rs, err := svc.build(target, in, tools.Configs["claude"], approvedRC(rc))

		// Assert
		require.NoError(t, err)
		assert.Contains(t, rs.Env, "AGENTIC_MARKETPLACES=acme")
	})

	t.Run("no marketplaces configured leaves AGENTIC_MARKETPLACES unset", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}

		// Act
		rs, err := New(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		for _, e := range rs.Env {
			assert.NotContains(t, e, "AGENTIC_MARKETPLACES")
		}
	})
}

func Test_Service_buildWithInstructions(t *testing.T) {
	d := &fakeDocker{}

	// BuildInstructions' own content rules (sections, formatting) are covered by
	// TestBuildInstructions, and PrepareInstructions' merge/sync-back behavior is
	// covered by TestPrepareInstructions - these subtests only cover what
	// BuildWithInstructions itself adds: threading content -> snapshot -> mount
	// into Build, and calling cleanup on a Build failure instead of leaking.

	t.Run("wires the generated content into the RunSpec's instructions mount", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}
		require.NoError(t, tools.Configs["claude"].Runtime.Setup(in.ToolHome))

		// Act
		rs, cleanup, err := New(d, nil).buildWithInstructions(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		t.Cleanup(cleanup)
		instructionsVolume := findVolumeSuffix(t, rs.Volumes, ":$CONTAINER_HOME/.claude/CLAUDE.md")
		got, err := os.ReadFile(mount.HostPart(instructionsVolume))
		require.NoError(t, err)
		assert.Contains(t, string(got), "# Agentic container environment")
	})

	t.Run("returned cleanup is wired to a real finalize, not a no-op", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}
		require.NoError(t, tools.Configs["claude"].Runtime.Setup(in.ToolHome))
		rs, cleanup, err := New(d, nil).buildWithInstructions(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))
		require.NoError(t, err)
		instructionsVolume := findVolumeSuffix(t, rs.Volumes, ":$CONTAINER_HOME/.claude/CLAUDE.md")
		snapshotPath := mount.HostPart(instructionsVolume)

		// Act
		cleanup()

		// Assert
		_, statErr := os.Stat(snapshotPath)
		assert.True(t, os.IsNotExist(statErr))
	})

	t.Run("build error finalizes and removes the snapshot immediately, rather than leaking it", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), Env: []string{"TOOL_HOME=nope"}}
		require.NoError(t, tools.Configs["claude"].Runtime.Setup(in.ToolHome))
		before, err := filepath.Glob(filepath.Join(os.TempDir(), "agentic-instructions-*"))
		require.NoError(t, err)

		// Act
		_, cleanup, err := New(d, nil).buildWithInstructions(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.Error(t, err)
		assert.NotPanics(t, func() { cleanup() })
		after, globErr := filepath.Glob(filepath.Join(os.TempDir(), "agentic-instructions-*"))
		require.NoError(t, globErr)
		assert.Len(t, after, len(before), "snapshot temp file should already be cleaned up when Build fails")
	})
}

func TestToolNeedsMarketplaceSync(t *testing.T) {
	t.Run("tool without marketplace support returns false", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Marketplaces: []config.RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}

		// Act
		needs := New(&fakeDocker{}, nil).ToolNeedsMarketplaceSync(tools.Configs["opencode"], rc, "opencode")

		// Assert
		assert.False(t, needs)
	})

	t.Run("marketplace-capable tool with no marketplaces configured returns false", func(t *testing.T) {
		// Act
		needs := New(&fakeDocker{}, nil).ToolNeedsMarketplaceSync(tools.Configs["claude"], &config.AgenticRC{}, "claude")

		// Assert
		assert.False(t, needs)
	})

	t.Run("marketplace-capable tool with marketplaces configured returns true", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Marketplaces: []config.RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}

		// Act
		needs := New(&fakeDocker{}, nil).ToolNeedsMarketplaceSync(tools.Configs["claude"], rc, "claude")

		// Assert
		assert.True(t, needs)
	})
}

func Test_Service_syncToolMarketplaces(t *testing.T) {
	acme := &config.AgenticRC{Marketplaces: []config.RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}
	synced := func(entries []marketplace.Entry, dirFor func(marketplace.Entry) string) ([]marketplace.Result, error) {
		return []marketplace.Result{{Entry: entries[0], Dir: dirFor(entries[0])}}, nil
	}

	t.Run("tool that doesn't need marketplace sync returns nil", func(t *testing.T) {
		// Act
		mounts, names, err := New(&fakeDocker{}, nil).syncToolMarketplaces("/home", "claude", tools.Configs["claude"], &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.Nil(t, mounts)
		assert.Nil(t, names)
	})

	t.Run("syncs configured marketplaces and returns mounts and names", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		var gotEntries []marketplace.Entry
		var gotDir string
		svc := newSyncingService(&fakeDocker{}, func(entries []marketplace.Entry, dirFor func(marketplace.Entry) string) ([]marketplace.Result, error) {
			gotEntries = entries
			gotDir = dirFor(entries[0])
			return []marketplace.Result{{Entry: entries[0], Dir: dirFor(entries[0])}}, nil
		})

		// Act
		mounts, names, err := svc.syncToolMarketplaces(home, "claude", tools.Configs["claude"], acme)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []marketplace.Entry{{Name: "acme", URL: "git@example.com:acme.git"}}, gotEntries)
		wantDirName := marketplace.CloneDirName("git@example.com:acme.git")
		assert.Equal(t, filepath.Join(home, "marketplaces", wantDirName), gotDir)
		assert.Equal(t, []string{tools.Configs["claude"].Runtime.MarketplaceMount("acme", "git@example.com:acme.git")}, mounts)
		assert.Equal(t, []string{"acme"}, names)
	})

	t.Run("sync error is wrapped with tool name", func(t *testing.T) {
		// Arrange
		svc := newSyncingService(&fakeDocker{}, func([]marketplace.Entry, func(marketplace.Entry) string) ([]marketplace.Result, error) {
			return nil, fmt.Errorf("clone failed")
		})

		// Act
		_, _, err := svc.syncToolMarketplaces(t.TempDir(), "claude", tools.Configs["claude"], acme)

		// Assert
		assert.ErrorContains(t, err, "claude")
		assert.ErrorContains(t, err, "clone failed")
	})

	t.Run("records usage of this project after a successful sync", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		cwd, err := os.Getwd()
		require.NoError(t, err)

		// Act
		_, _, err = newSyncingService(&fakeDocker{}, synced).syncToolMarketplaces(home, "claude", tools.Configs["claude"], acme)

		// Assert
		require.NoError(t, err)
		reg, err := marketplace.LoadRegistry(filepath.Join(home, "marketplaces"))
		require.NoError(t, err)
		entries := reg.Marketplaces[marketplace.CloneDirName("git@example.com:acme.git")]
		require.Len(t, entries, 1)
		assert.Equal(t, []string{cwd}, entries[0].Projects)
	})

	t.Run("usage recording failure does not fail the run", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(home, "marketplaces"), nil, 0o600), "a file where the dir should be")

		// Act
		mounts, names, err := newSyncingService(&fakeDocker{}, synced).syncToolMarketplaces(home, "claude", tools.Configs["claude"], acme)

		// Assert
		require.NoError(t, err)
		assert.Len(t, mounts, 1)
		assert.Equal(t, []string{"acme"}, names)
	})
}

func Test_marketplaceEntries(t *testing.T) {
	// Arrange
	rc := &config.AgenticRC{Marketplaces: []config.RCMarketplace{
		{Name: "acme", URL: "git@example.test:acme.git"},
		{Name: "copilot-only", URL: "git@example.test:other.git", Tools: []string{"copilot"}},
	}}

	// Act
	entries := marketplaceEntries(rc, "claude")

	// Assert
	assert.Equal(t, []marketplace.Entry{{Name: "acme", URL: "git@example.test:acme.git"}}, entries)
}

func Test_toolMarketplaceMounts(t *testing.T) {
	t.Run("returns each result's mount spec and name", func(t *testing.T) {
		// Arrange
		results := []marketplace.Result{{Entry: marketplace.Entry{Name: "acme", URL: "git@example.test:acme.git"}}}

		// Act
		mounts, names := toolMarketplaceMounts(tools.Configs["claude"], results)

		// Assert
		assert.Equal(t, []string{tools.Configs["claude"].Runtime.MarketplaceMount("acme", "git@example.test:acme.git")}, mounts)
		assert.Equal(t, []string{"acme"}, names)
	})

	t.Run("warns about a stale clone and still mounts it", func(t *testing.T) {
		// Arrange
		logs := stubErrLog(t)
		results := []marketplace.Result{{Entry: marketplace.Entry{Name: "acme", URL: "git@example.test:acme.git"}, Stale: true, Warning: fmt.Errorf("offline")}}

		// Act
		mounts, names := toolMarketplaceMounts(tools.Configs["claude"], results)

		// Assert
		assert.Len(t, mounts, 1)
		assert.Equal(t, []string{"acme"}, names)
		assert.Contains(t, logs.String(), `marketplace "acme": offline; using existing clone`)
	})
}

func Test_validateEnv(t *testing.T) {
	t.Run("accepts ordinary KEY=VALUE", func(t *testing.T) {
		// Act
		err := validateEnv([]string{"MAVEN_OPTS=-Dfoo=bar"}, false, false)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("accepts bare KEY", func(t *testing.T) {
		// Act
		err := validateEnv([]string{"CI"}, false, false)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("accepts overriding terminal vars", func(t *testing.T) {
		// Act
		err := validateEnv([]string{"NO_COLOR=1", "TERM=dumb"}, true, false)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("accepts proxy var when proxy disabled", func(t *testing.T) {
		// Act
		err := validateEnv([]string{"HTTP_PROXY=http://my-corp-proxy"}, false, false)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("rejects proxy var when proxy enabled", func(t *testing.T) {
		// Act
		err := validateEnv([]string{"HTTP_PROXY=http://evil"}, true, false)

		// Assert
		assert.ErrorContains(t, err, "HTTP_PROXY")
	})

	t.Run("accepts docker var when dind disabled", func(t *testing.T) {
		// Act
		err := validateEnv([]string{"DOCKER_HOST=tcp://build-host:2376"}, false, false)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("rejects docker var when dind enabled", func(t *testing.T) {
		// Act
		err := validateEnv([]string{"DOCKER_CERT_PATH=/tmp"}, false, true)

		// Assert
		assert.ErrorContains(t, err, "DOCKER_CERT_PATH")
	})

	t.Run("rejects bare reserved name regardless of proxy", func(t *testing.T) {
		// Act
		err := validateEnv([]string{"TOOL_HOME"}, false, false)

		// Assert
		assert.ErrorContains(t, err, "TOOL_HOME")
	})
}

func Test_runVolumes(t *testing.T) {
	// Arrange
	toolConfig := tools.ToolConfig{Runtime: tools.RuntimeConfig{Mounts: func() []string { return []string{"/tool:/tool"} }}}
	in := Input{
		Volumes:           []string{"/flag:/flag:rw"},
		InstructionsMount: "/snapshot.md:/instructions.md",
	}
	req := buildRequest{in: in, toolConfig: toolConfig, rc: &config.AgenticRC{}}

	// Act
	result := runVolumes(req, []string{"/marketplace:/marketplace"})

	// Assert
	assert.Equal(t, []string{
		"/tool:/tool",
		"/flag:/flag:rw",
		"/marketplace:/marketplace",
		"/snapshot.md:/instructions.md",
	}, result)
}
