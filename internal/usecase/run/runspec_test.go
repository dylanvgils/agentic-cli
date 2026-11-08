package run

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
	"github.com/dylanvgils/agentic-cli/internal/network/credentials"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Service_build(t *testing.T) {
	trusted := func(string) (*docker.ImageInfo, error) { return &docker.ImageInfo{ProxyTrust: true}, nil }
	d := &fakeDocker{inspectImage: trusted}

	t.Run("wires the target, input and config into the spec", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude", ToolArgs: []string{"--from-cli"}}
		in := Input{
			ToolHome: t.TempDir(),
			Volumes:  []string{"/host:/container:rw"},
			DryRun:   true,
		}
		rc := &config.AgenticRC{Run: config.RCRun{
			RCLimits: config.RCLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"},
			Args:     map[string][]string{"claude": {"--from-rc"}},
		}}

		// Act
		rs, err := newTestService(d, nil).build(target, in, tools.Configs["claude"], approvedRC(rc))

		// Assert
		require.NoError(t, err)
		assert.Contains(t, rs.Volumes, "/host:/container:rw")
		assert.Equal(t, platform.HostUser(), *rs.User)
		assert.Equal(t, iostreams.IsTerminal(), rs.TTY, "a TTY only when stdin is a terminal")
		assert.Equal(t, in.ToolHome, rs.ToolHome)
		assert.True(t, rs.DryRun)
		assert.NotEmpty(t, rs.EphemeralDirs)
		assert.Equal(t, []string{"--from-rc", "--from-cli"}, rs.ToolArgs)
		assert.Equal(t, sandbox.ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}, rs.Limits)
		assert.Equal(t, rs.Limits, rs.Dind.Limits, "dind limits inherit the tool's")
		assert.Equal(t, sandbox.EgressOff, rs.Egress.Mode)
		for _, v := range rs.Volumes {
			assert.NotContains(t, v, "CLAUDE.md", "no instructions mount without one in the input")
		}
		for _, e := range rs.Env {
			assert.NotContains(t, e, "AGENTIC_MARKETPLACES", "no marketplaces configured")
		}
	})

	t.Run("skip entrypoint wired", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude", SkipEntrypoint: true}
		in := Input{ToolHome: t.TempDir()}

		// Act
		rs, err := newTestService(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		assert.True(t, rs.SkipEntrypoint)
	})

	t.Run("instructions mount wired after the tool's base mounts", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), InstructionsMount: "/tmp/snapshot.md:$CONTAINER_HOME/.claude/CLAUDE.md"}

		// Act
		rs, err := newTestService(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		require.Contains(t, rs.Volumes, "/tmp/snapshot.md:$CONTAINER_HOME/.claude/CLAUDE.md")
		baseMountIdx := slices.Index(rs.Volumes, tools.Configs["claude"].Runtime.Mounts()[1])
		instructionsMountIdx := slices.Index(rs.Volumes, "/tmp/snapshot.md:$CONTAINER_HOME/.claude/CLAUDE.md")
		assert.Less(t, baseMountIdx, instructionsMountIdx, "instructions mount must overlay the base directory mount, so it must be listed after it")
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
		rs, err := newTestService(d, nil).build(target, in, tools.Configs["claude"], approvedRC(rc))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.ResourceLimits{PidsLimit: "4096", CPUs: "2", Memory: "16g"}, rs.Dind.Limits)
		assert.Equal(t, "2g", rs.Limits.Memory, "tool limit is unaffected")
	})

	t.Run("config mount through a planted symlink out of cwd is refused", func(t *testing.T) {
		// Arrange
		chdirSymlinkWorkspace(t)
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}
		rc := &config.AgenticRC{Run: config.RCRun{Mounts: []string{"$PWD/escape:/workspace/escape"}}}

		// Act
		_, err := newTestService(d, nil).build(target, in, tools.Configs["claude"], approvedRC(rc))

		// Assert
		assert.ErrorContains(t, err, "goes through workspace symlink")
	})

	t.Run("proxy wired with merged allowlist and image", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), ProxyMode: sandbox.EgressEnforce}
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{AllowedHosts: []string{"extra.example.com"}}}}

		// Act
		rs, err := newTestService(d, nil).build(target, in, tools.Configs["claude"], approvedRC(rc))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.EgressEnforce, rs.Egress.Mode)
		assert.Equal(t, []string{".anthropic.com", ".claude.ai", ".claude.com", "extra.example.com"}, rs.Egress.WholeHosts())
	})

	t.Run("proxy gets path rules after the hosts allowed whole", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), ProxyMode: sandbox.EgressEnforce}
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{AllowedPaths: []config.RCAllowedPath{
			{Host: "raw.example.test", Paths: []string{"/org/repo/**"}},
		}}}}

		// Act
		rs, err := newTestService(d, nil).build(target, in, tools.Configs["claude"], approvedRC(rc))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.AllowRule{Host: "raw.example.test", Paths: []string{"/org/repo/**"}}, rs.Egress.Rules[len(rs.Egress.Rules)-1])
	})

	t.Run("path rules refuse an image that can't trust the proxy CA", func(t *testing.T) {
		// Arrange
		untrusted := &fakeDocker{inspectImage: func(string) (*docker.ImageInfo, error) { return &docker.ImageInfo{}, nil }}
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), ProxyMode: sandbox.EgressEnforce}
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{AllowedPaths: []config.RCAllowedPath{
			{Host: "raw.example.test", Paths: []string{"/org/repo/**"}},
		}}}}

		// Act
		_, err := newTestService(untrusted, nil).build(target, in, tools.Configs["claude"], approvedRC(rc))

		// Assert
		assert.ErrorContains(t, err, "can't trust the proxy CA")
	})

	t.Run("path rules limit a host the tool's baseline allows whole", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), ProxyMode: sandbox.EgressEnforce}
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{AllowedPaths: []config.RCAllowedPath{
			{Host: "api.anthropic.com", Paths: []string{"/v1/**"}},
		}}}}

		// Act
		rs, err := newTestService(d, nil).build(target, in, tools.Configs["claude"], approvedRC(rc))

		// Assert
		require.NoError(t, err)
		assert.Contains(t, rs.Egress.Rules, sandbox.AllowRule{Host: "api.anthropic.com", Paths: []string{"/v1/**"}})
		assert.Contains(t, rs.Egress.WholeHosts(), ".anthropic.com", "the proxy lets the path rules win over the wildcard")
	})

	t.Run("proxy monitor wired", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), ProxyMode: sandbox.EgressMonitor}

		// Act
		rs, err := newTestService(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.EgressMonitor, rs.Egress.Mode)
	})

	for _, tc := range []struct {
		name string
		svc  *Service
		in   Input
	}{
		{
			name: "proxy credentials wired for the docker runtime to place",
			svc:  newTestService(d, nil),
			in:   Input{ProxyMode: sandbox.EgressEnforce},
		},
		{
			name: "microsandbox credentials wired without the proxy, placeholder env or image inspect",
			svc:  newTestService(&fakeDocker{}, &fakeMicrosandbox{}),
			in:   Input{Runtime: sandbox.RuntimeMicrosandbox},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			target := Target{ToolName: "claude", ImageName: "agentic-claude"}
			in := tc.in
			in.ToolHome = t.TempDir()
			in.Credentials = []credentials.Resolved{{
				Hosts:   []string{"api.example.test"},
				Headers: []string{"X-Api-Key"},
				Env:     []string{"EXAMPLE_API_KEY"},
				Secret:  "test-secret",
			}}

			// Act
			rs, err := tc.svc.build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

			// Assert
			require.NoError(t, err)
			assert.Equal(t, []sandbox.Credential{
				{Name: "EXAMPLE_API_KEY", Aliases: []string{}, Hosts: []string{"api.example.test"}, Headers: []string{"X-Api-Key"}, Secret: "test-secret"},
			}, rs.Credentials)
			assert.Equal(t, tc.in.ProxyMode, rs.Egress.Mode)
		})
	}

	t.Run("microsandbox allows path rule hosts whole", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), Runtime: sandbox.RuntimeMicrosandbox, ProxyMode: sandbox.EgressEnforce}
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{AllowedPaths: []config.RCAllowedPath{
			{Host: "raw.example.test", Paths: []string{"/org/repo/**"}},
		}}}}

		// Act
		rs, err := newTestService(&fakeDocker{}, &fakeMicrosandbox{}).build(target, in, tools.Configs["claude"], approvedRC(rc))

		// Assert
		require.NoError(t, err)
		assert.False(t, rs.Egress.HasPathRules())
		assert.Contains(t, rs.Egress.WholeHosts(), "raw.example.test")
	})

	t.Run("proxy credentials refused on an image without proxy trust", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) { return &docker.ImageInfo{}, nil },
		}
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		resolved := credentials.Resolved{Hosts: []string{"api.example.test"}, Env: []string{"EXAMPLE_API_KEY"}}
		in := Input{ToolHome: t.TempDir(), ProxyMode: sandbox.EgressEnforce, Credentials: []credentials.Resolved{resolved}}

		// Act
		_, err := newTestService(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

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
			ProxyMode:   sandbox.EgressEnforce,
			Credentials: []credentials.Resolved{{Path: filepath.Join(dir, "token")}},
		}

		// Act
		_, err := newTestService(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		assert.ErrorContains(t, err, "which the tool container can access")
	})

	t.Run("dind wired with sidecar image and docker hub allowlisted", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: trusted}
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), ProxyMode: sandbox.EgressEnforce, DindEnabled: true}

		// Act
		rs, err := newTestService(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert - agentic-net is skipped since the tool joins a per-run network
		require.NoError(t, err)
		assert.True(t, rs.Dind.Enabled)
		assert.Contains(t, rs.Egress.WholeHosts(), "registry-1.docker.io")
	})

	t.Run("dind rejects env overriding the docker host", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir(), DindEnabled: true, Env: []string{"DOCKER_HOST=unix:///var/run/docker.sock"}}

		// Act
		_, err := newTestService(d, nil).build(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

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
		rs, cleanup, err := newTestService(d, nil).buildWithInstructions(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.NoError(t, err)
		t.Cleanup(cleanup)
		instructionsVolume := findVolumeSuffix(t, rs.Volumes, ":$CONTAINER_HOME/.claude/CLAUDE.md")
		got, err := os.ReadFile(sandbox.HostPart(instructionsVolume))
		require.NoError(t, err)
		assert.Contains(t, string(got), "# Agentic container environment")
	})

	t.Run("returned cleanup is wired to a real finalize, not a no-op", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		in := Input{ToolHome: t.TempDir()}
		require.NoError(t, tools.Configs["claude"].Runtime.Setup(in.ToolHome))
		rs, cleanup, err := newTestService(d, nil).buildWithInstructions(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))
		require.NoError(t, err)
		instructionsVolume := findVolumeSuffix(t, rs.Volumes, ":$CONTAINER_HOME/.claude/CLAUDE.md")
		snapshotPath := sandbox.HostPart(instructionsVolume)

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
		_, cleanup, err := newTestService(d, nil).buildWithInstructions(target, in, tools.Configs["claude"], approvedRC(&config.AgenticRC{}))

		// Assert
		require.Error(t, err)
		assert.NotPanics(t, func() { cleanup() })
		after, globErr := filepath.Glob(filepath.Join(os.TempDir(), "agentic-instructions-*"))
		require.NoError(t, globErr)
		assert.Len(t, after, len(before), "snapshot temp file should already be cleaned up when Build fails")
	})
}

func TestToolNeedsMarketplaceSync(t *testing.T) {
	acme := &config.AgenticRC{Marketplaces: []config.RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}

	for _, tc := range []struct {
		name string
		tool string
		rc   *config.AgenticRC
		want bool
	}{
		{name: "tool without marketplace support returns false", tool: "opencode", rc: acme},
		{name: "marketplace-capable tool with no marketplaces configured returns false", tool: "claude", rc: &config.AgenticRC{}},
		{name: "marketplace-capable tool with marketplaces configured returns true", tool: "claude", rc: acme, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			needs := newTestService(&fakeDocker{}, nil).ToolNeedsMarketplaceSync(tools.Configs[tc.tool], tc.rc, tc.tool)

			// Assert
			assert.Equal(t, tc.want, needs)
		})
	}
}

func Test_Service_syncToolMarketplaces(t *testing.T) {
	acme := &config.AgenticRC{Marketplaces: []config.RCMarketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}
	synced := func(entries []marketplace.Entry, dirFor func(marketplace.Entry) string) ([]marketplace.Result, error) {
		return []marketplace.Result{{Entry: entries[0], Dir: dirFor(entries[0])}}, nil
	}

	t.Run("tool that doesn't need marketplace sync returns nil", func(t *testing.T) {
		// Act
		mounts, names, err := newTestService(&fakeDocker{}, nil).syncToolMarketplaces("/home", "claude", tools.Configs["claude"], &config.AgenticRC{})

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
		logs := iostreamstest.StubErr(t)
		results := []marketplace.Result{{Entry: marketplace.Entry{Name: "acme", URL: "git@example.test:acme.git"}, Stale: true, Warning: fmt.Errorf("offline")}}

		// Act
		mounts, names := toolMarketplaceMounts(tools.Configs["claude"], results)

		// Assert
		assert.Len(t, mounts, 1)
		assert.Equal(t, []string{"acme"}, names)
		assert.Contains(t, logs.String(), `marketplace "acme": offline; using existing clone`)
	})
}

func Test_isReservedEnvName(t *testing.T) {
	t.Run("an integration's env is reserved only with that integration", func(t *testing.T) {
		// Act
		without, with := isReservedEnvName("MAVEN_ARGS", true, false, nil), isReservedEnvName("MAVEN_ARGS", true, false, []string{"maven"})

		// Assert
		assert.False(t, without)
		assert.True(t, with)
	})

	t.Run("agentic's own env is reserved", func(t *testing.T) {
		// Act
		reserved := isReservedEnvName("HTTPS_PROXY", true, false, nil)

		// Assert
		assert.True(t, reserved)
	})
}

func Test_validateEnv(t *testing.T) {
	for _, tc := range []struct {
		name         string
		env          string
		proxy        bool
		dind         bool
		integrations []string
		wantErr      string
	}{
		{name: "accepts ordinary KEY=VALUE", env: "MAVEN_OPTS=-Dfoo=bar"},
		{name: "accepts bare KEY", env: "CI"},
		{name: "accepts proxy var when proxy disabled", env: "HTTP_PROXY=http://my-corp-proxy"},
		{name: "rejects proxy var when proxy enabled", env: "HTTP_PROXY=http://evil", proxy: true, wantErr: "HTTP_PROXY"},
		{name: "accepts docker var when dind disabled", env: "DOCKER_HOST=tcp://build-host:2376"},
		{name: "rejects docker var when dind enabled", env: "DOCKER_CERT_PATH=/tmp", dind: true, wantErr: "DOCKER_CERT_PATH"},
		{name: "rejects a var a proxy integration sets", env: "MAVEN_ARGS=--quiet", proxy: true, integrations: []string{"maven"}, wantErr: `"MAVEN_ARGS" is managed by agentic`},
		{name: "accepts a var no proxy integration sets", env: "MAVEN_ARGS=--quiet", proxy: true},
		{name: "rejects bare reserved name regardless of proxy", env: "TOOL_HOME", wantErr: "TOOL_HOME"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			err := validateEnv([]string{tc.env}, tc.proxy, tc.dind, tc.integrations)

			// Assert
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
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
