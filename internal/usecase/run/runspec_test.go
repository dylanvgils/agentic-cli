package run

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config/agenticrc"
	"github.com/dylanvgils/agentic-cli/internal/config/precedence"
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
		flags := Flags{
			ToolHome: t.TempDir(),
			Volumes:  []string{"/host:/container:rw"},
			Ports:    []string{"3000"},
			Proxy:    precedence.ProxyInput{MonitorFlag: true},
			DryRun:   true,
		}
		rc := &agenticrc.Config{Run: agenticrc.Run{
			Limits: agenticrc.Limits{PidsLimit: "512", CPUs: "2", Memory: "2g"},
			Args:   map[string][]string{"claude": {"--from-rc"}},
		}}
		svc := newTestService(d, nil)
		c := resolvedConfig(t, svc, target, flags, rc)

		// Act
		rs, err := svc.build(c)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, rs.Mounts, sandbox.Mount{Source: "/host", Target: "/container", Options: []string{"rw"}})
		assert.Equal(t, []sandbox.PortMapping{{Host: 3000, Guest: 3000}}, rs.Ports)
		assert.Equal(t, platform.HostUser(), *rs.User)
		assert.Equal(t, iostreams.IsTerminal(), rs.TTY, "a TTY only when stdin is a terminal")
		assert.Equal(t, flags.ToolHome, rs.ToolHome)
		assert.NotEmpty(t, rs.EphemeralDirs)
		assert.Equal(t, []string{"--from-rc", "--from-cli"}, rs.ToolArgs)
		assert.Equal(t, sandbox.ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"}, rs.Limits)
		assert.Equal(t, rs.Limits, rs.Dind.Limits, "dind limits inherit the tool's")
		assert.Equal(t, sandbox.EgressMonitor, rs.Egress.Mode)
		for _, m := range rs.Mounts {
			assert.NotContains(t, m.Target, "CLAUDE.md", "no instructions mount without one in the config")
		}
		for _, e := range rs.Env {
			assert.NotContains(t, e, "AGENTIC_MARKETPLACES", "no marketplaces configured")
		}
	})

	t.Run("config mount through a planted symlink out of cwd is refused", func(t *testing.T) {
		// Arrange
		chdirSymlinkWorkspace(t)
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		rc := &agenticrc.Config{Run: agenticrc.Run{Mounts: []string{"$PWD/escape:/workspace/escape"}}}

		svc := newTestService(d, nil)
		c := resolvedConfig(t, svc, target, Flags{ToolHome: t.TempDir()}, rc)

		// Act
		_, err := svc.build(c)

		// Assert
		assert.ErrorContains(t, err, "goes through workspace symlink")
	})

	t.Run("proxy wired with the resolved rules", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude", SkipEntrypoint: true}
		rc := &agenticrc.Config{Run: agenticrc.Run{Proxy: agenticrc.Proxy{AllowedPaths: []agenticrc.AllowedPath{
			{Host: "raw.example.test", Paths: []string{"/org/repo/**"}},
		}}}}

		svc := newTestService(d, nil)
		flags := Flags{
			ToolHome: t.TempDir(),
			Proxy:    precedence.ProxyInput{ProxyFlag: true},
		}
		c := resolvedConfig(t, svc, target, flags, rc)

		// Act
		rs, err := svc.build(c)

		// Assert
		require.NoError(t, err)
		assert.True(t, rs.SkipEntrypoint)
		assert.Equal(t, sandbox.EgressEnforce, rs.Egress.Mode)
		assert.Equal(t, sandbox.AllowRule{Host: "raw.example.test", Paths: []string{"/org/repo/**"}}, rs.Egress.Rules[len(rs.Egress.Rules)-1])
	})

	t.Run("path rules refuse an image that can't trust the proxy CA", func(t *testing.T) {
		// Arrange
		untrusted := &fakeDocker{inspectImage: func(string) (*docker.ImageInfo, error) { return &docker.ImageInfo{}, nil }}
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		rc := &agenticrc.Config{Run: agenticrc.Run{Proxy: agenticrc.Proxy{AllowedPaths: []agenticrc.AllowedPath{
			{Host: "raw.example.test", Paths: []string{"/org/repo/**"}},
		}}}}

		svc := newTestService(untrusted, nil)
		flags := Flags{
			ToolHome: t.TempDir(),
			Proxy:    precedence.ProxyInput{ProxyFlag: true},
		}
		c := resolvedConfig(t, svc, target, flags, rc)

		// Act
		_, err := svc.build(c)

		// Assert
		assert.ErrorContains(t, err, "can't trust the proxy CA")
	})

	for _, tc := range []struct {
		name     string
		svc      *Service
		flags    Flags
		wantMode sandbox.EgressMode
	}{
		{
			name:     "proxy credentials wired for the docker runtime to place",
			svc:      newTestService(d, nil),
			flags:    Flags{Proxy: precedence.ProxyInput{ProxyFlag: true}},
			wantMode: sandbox.EgressEnforce,
		},
		{
			name:     "microsandbox credentials wired without the proxy, placeholder env or image inspect",
			svc:      newTestService(&fakeDocker{}, &fakeMicrosandbox{}),
			flags:    Flags{Runtime: sandbox.RuntimeMicrosandbox},
			wantMode: sandbox.EgressOff,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			target := Target{ToolName: "claude", ImageName: "agentic-claude"}
			flags := tc.flags
			flags.ToolHome = t.TempDir()
			c := resolvedConfig(t, tc.svc, target, flags, &agenticrc.Config{})
			c.Egress.Credentials = []credentials.Resolved{{
				Hosts:   []string{"api.example.test"},
				Headers: []string{"X-Api-Key"},
				Env:     []string{"EXAMPLE_API_KEY"},
				Secret:  "test-secret",
			}}

			// Act
			rs, err := tc.svc.build(c)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, []sandbox.Credential{
				{Name: "EXAMPLE_API_KEY", Aliases: []string{}, Hosts: []string{"api.example.test"}, Headers: []string{"X-Api-Key"}, Secret: "test-secret"},
			}, rs.Credentials)
			assert.Equal(t, tc.wantMode, rs.Egress.Mode)
		})
	}

	t.Run("proxy credentials refused on an image without proxy trust", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) { return &docker.ImageInfo{}, nil },
		}
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		resolved := credentials.Resolved{Hosts: []string{"api.example.test"}, Env: []string{"EXAMPLE_API_KEY"}}

		svc := newTestService(d, nil)
		flags := Flags{
			ToolHome: t.TempDir(),
			Proxy:    precedence.ProxyInput{ProxyFlag: true},
		}
		c := resolvedConfig(t, svc, target, flags, &agenticrc.Config{})
		c.Egress.Credentials = []credentials.Resolved{resolved}

		// Act
		_, err := svc.build(c)

		// Assert
		assert.ErrorContains(t, err, "agentic update claude")
	})

	t.Run("proxy credentials rejected when the secret is mounted", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		dir := t.TempDir()
		flags := Flags{
			ToolHome: t.TempDir(),
			Volumes:  []string{dir + ":/data"},
			Proxy:    precedence.ProxyInput{ProxyFlag: true},
		}
		svc := newTestService(d, nil)
		c := resolvedConfig(t, svc, target, flags, &agenticrc.Config{})
		c.Egress.Credentials = []credentials.Resolved{{Path: filepath.Join(dir, "token")}}

		// Act
		_, err := svc.build(c)

		// Assert
		assert.ErrorContains(t, err, "which the tool container can access")
	})

	t.Run("dind rejects env overriding the docker host", func(t *testing.T) {
		// Arrange
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}

		svc := newTestService(d, nil)
		flags := Flags{
			ToolHome: t.TempDir(),
			Dind:     precedence.DindInput{DindFlag: true},
			Env:      []string{"DOCKER_HOST=unix:///var/run/docker.sock"},
		}
		c := resolvedConfig(t, svc, target, flags, &agenticrc.Config{})

		// Act
		_, err := svc.build(c)

		// Assert
		assert.ErrorContains(t, err, "DOCKER_HOST")
	})

	t.Run("marketplace names wired into AGENTIC_MARKETPLACES env", func(t *testing.T) {
		// Arrange
		svc := newSyncingService(d, func(entries []marketplace.Entry, dirFor func(marketplace.Entry) string) ([]marketplace.Result, error) {
			return []marketplace.Result{{Entry: entries[0], Dir: dirFor(entries[0])}}, nil
		})
		target := Target{ToolName: "claude", ImageName: "agentic-claude"}
		rc := &agenticrc.Config{Marketplaces: []agenticrc.Marketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}

		c := resolvedConfig(t, svc, target, Flags{ToolHome: t.TempDir()}, rc)

		// Act
		rs, err := svc.build(c)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, rs.Env, "AGENTIC_MARKETPLACES=acme")
	})
}

func Test_Service_buildWithInstructions(t *testing.T) {
	// Arrange
	stubRunTempDir(t)
	c := newTestRunConfig(t)
	require.NoError(t, c.Tool.Runtime.Setup(c.Homes.Tool))

	// Act
	rs, err := newTestService(&fakeDocker{}, nil).buildWithInstructions(testRunID, c)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, testRunID, rs.RunID)
	instructions := findMountTarget(t, rs.Mounts, "/home/agent/.claude/CLAUDE.md")
	got, err := os.ReadFile(instructions.Source)
	require.NoError(t, err)
	assert.Contains(t, string(got), "# Agentic container environment")
}

func TestToolNeedsMarketplaceSync(t *testing.T) {
	acme := &agenticrc.Config{Marketplaces: []agenticrc.Marketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}

	for _, tc := range []struct {
		name string
		tool string
		rc   *agenticrc.Config
		want bool
	}{
		{name: "tool without marketplace support returns false", tool: "opencode", rc: acme},
		{name: "marketplace-capable tool with no marketplaces configured returns false", tool: "claude", rc: &agenticrc.Config{}},
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
	acme := &agenticrc.Config{Marketplaces: []agenticrc.Marketplace{{Name: "acme", URL: "git@example.com:acme.git"}}}
	synced := func(entries []marketplace.Entry, dirFor func(marketplace.Entry) string) ([]marketplace.Result, error) {
		return []marketplace.Result{{Entry: entries[0], Dir: dirFor(entries[0])}}, nil
	}

	t.Run("tool that doesn't need marketplace sync returns nil", func(t *testing.T) {
		// Act
		mounts, names, err := newTestService(&fakeDocker{}, nil).syncToolMarketplaces("/home", "claude", tools.Configs["claude"], &agenticrc.Config{})

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
		require.ErrorContains(t, err, "claude")
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

func Test_toolMarketplaceMounts(t *testing.T) {
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

func Test_runMountList(t *testing.T) {
	// Arrange
	base := sandbox.Mount{Source: "/flag", Target: "/flag"}
	c := runConfig{
		Homes: homes{Tool: "/tool-home", Container: "/home/agent"},
		Mounts: runMounts{
			Base:         []sandbox.Mount{base},
			Instructions: "/snapshot.md:$CONTAINER_HOME/instructions.md",
		},
	}

	// Act
	result, err := runMountList(c, []string{"$TOOL_HOME/marketplace:/marketplace:ro"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []sandbox.Mount{
		base,
		{Source: "/tool-home/marketplace", Target: "/marketplace", ReadOnly: true},
		{Source: "/snapshot.md", Target: "/home/agent/instructions.md"},
	}, result)
}
