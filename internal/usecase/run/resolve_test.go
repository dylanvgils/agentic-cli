package run

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Resolve(t *testing.T) {
	built := func(string) (*docker.ImageInfo, error) {
		return &docker.ImageInfo{Image: "agentic-claude", Version: "1.0.0", ProxyTrust: true}, nil
	}

	// dueUpdate offers an update from 1.0.0 to 2.0.0 through the prompter, as a due update check does
	dueUpdate := &fakeToolUpdates{offer: func(u ToolUpdate, confirm func(tool, installed, latest string) bool) error {
		confirm(u.Tool, "1.0.0", "2.0.0")
		return nil
	}}

	t.Run("starts nothing", func(t *testing.T) {
		// Arrange
		t.Setenv("TMPDIR", t.TempDir())
		req := newResolveRequest(t)
		w := &fakeWatchdog{}
		svc := New(&fakeDocker{inspectImage: built}, nil, &fakeToolUpdates{}, &fakeSidecarImages{}, w, &fakeCommandLog{})

		// Act
		_, err := svc.Resolve(req, &fakePrompter{})

		// Assert
		require.NoError(t, err)
		assert.Empty(t, w.spawned)
		entries, err := os.ReadDir(os.TempDir())
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	for _, tc := range []struct {
		name    string
		request func(t *testing.T) Request
		docker  *fakeDocker
		wantErr string
	}{
		{
			name:    "missing image stops before any prompt",
			request: newResolveRequest,
			docker:  &fakeDocker{},
			wantErr: "agentic build claude",
		},
		{
			name: "a port in use stops before any prompt",
			request: func(t *testing.T) Request {
				req := newResolveRequest(t)
				req.Flags.Ports = []string{strconv.Itoa(int(busyPort(t)))}
				return req
			},
			docker:  &fakeDocker{inspectImage: built},
			wantErr: "is in use",
		},
		{
			name: "a credential without env stops before any prompt",
			request: func(t *testing.T) Request {
				req := newCredentialRequest(t)
				req.Project.RC.Run.Proxy.Credentials[0].Env = nil
				return req
			},
			docker:  &fakeDocker{inspectImage: built},
			wantErr: "needs env, as the tool must send the placeholder",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			req := tc.request(t)
			p := &fakePrompter{}

			// Act
			_, err := newTestService(tc.docker, nil).Resolve(req, p)

			// Assert
			require.ErrorContains(t, err, tc.wantErr)
			assert.Empty(t, p.trustAsked)
		})
	}

	t.Run("missing msb stops before the image check and any prompt", func(t *testing.T) {
		// Arrange
		req := newResolveRequest(t)
		req.Flags.Runtime = sandbox.RuntimeMicrosandbox
		inspected := false
		d := &fakeDocker{inspectImage: func(name string) (*docker.ImageInfo, error) {
			inspected = true
			return built(name)
		}}
		p := &fakePrompter{}
		m := &fakeMicrosandbox{available: func() error { return errors.New("msb not found on PATH") }}

		// Act
		_, err := newTestService(d, m).Resolve(req, p)

		// Assert
		require.EqualError(t, err, "msb not found on PATH")
		assert.False(t, inspected)
		assert.Empty(t, p.trustAsked)
	})

	t.Run("refused trust stops before the tool update and setup", func(t *testing.T) {
		// Arrange
		req := newResolveRequest(t)
		iostreamstest.StubOut(t)
		tool := tools.Configs["claude"]
		setUp := false
		tool.Runtime.Setup = func(string) error {
			setUp = true
			return nil
		}
		req.Tool = tool
		p := &fakePrompter{trustDir: func(string) error { return errors.New("directory not trusted") }}

		// Act
		_, err := newUpdatingService(&fakeDocker{inspectImage: built}, dueUpdate).Resolve(req, p)

		// Assert
		require.EqualError(t, err, "directory not trusted")
		assert.Empty(t, p.updatesOffered)
		assert.False(t, setUp)
	})

	t.Run("offers the tool update through the prompter", func(t *testing.T) {
		// Arrange
		req := newResolveRequest(t)
		iostreamstest.StubOut(t)
		var got ToolUpdate
		u := &fakeToolUpdates{offer: func(update ToolUpdate, confirm func(tool, installed, latest string) bool) error {
			got = update
			confirm(update.Tool, "1.0.0", "2.0.0")
			return nil
		}}
		p := &fakePrompter{}

		// Act
		_, err := newUpdatingService(&fakeDocker{inspectImage: built}, u).Resolve(req, p)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, ToolUpdate{Home: req.Flags.ToolHome, RC: req.Project.RC, Tool: "claude", Image: "agentic-claude"}, got)
		assert.Equal(t, []string{"claude"}, p.updatesOffered)
	})

	t.Run("a failed tool update stops the run", func(t *testing.T) {
		// Arrange
		req := newResolveRequest(t)
		iostreamstest.StubOut(t)
		u := &fakeToolUpdates{offer: func(ToolUpdate, func(string, string, string) bool) error { return errors.New("update failed") }}

		// Act
		_, err := newUpdatingService(&fakeDocker{inspectImage: built}, u).Resolve(req, &fakePrompter{})

		// Assert
		require.EqualError(t, err, "update failed")
	})

	t.Run("dry run offers no tool update", func(t *testing.T) {
		// Arrange
		req := newResolveRequest(t)
		req.Flags.DryRun = true
		iostreamstest.StubOut(t)
		p := &fakePrompter{}

		// Act
		_, err := newUpdatingService(&fakeDocker{inspectImage: built}, dueUpdate).Resolve(req, p)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, p.updatesOffered)
	})

	t.Run("tool setup error is wrapped with the tool name", func(t *testing.T) {
		// Arrange
		req := newResolveRequest(t)
		tool := tools.Configs["claude"]
		tool.Runtime.Setup = func(string) error { return errors.New("disk full") }
		req.Tool = tool

		// Act
		_, err := newTestService(&fakeDocker{inspectImage: built}, nil).Resolve(req, &fakePrompter{})

		// Assert
		require.EqualError(t, err, "setup claude: disk full")
	})

	t.Run("refused settings stop before the tool update", func(t *testing.T) {
		// Arrange
		req := newResolveRequest(t)
		iostreamstest.StubOut(t)
		rc := &config.AgenticRC{}
		rc.Run.Mounts = []string{"~/.example:/x:rw"}
		req.Project.Layers = []config.RCLayer{{Path: filepath.Join(req.Project.WorkDir, ".agenticrc.toml"), RC: rc}}
		p := &fakePrompter{approveSettings: func(config.RCLayer, []config.GuardedSetting) error { return errors.New("settings not approved") }}

		// Act
		_, err := newUpdatingService(&fakeDocker{inspectImage: built}, dueUpdate).Resolve(req, p)

		// Assert
		require.EqualError(t, err, "settings not approved")
		assert.Empty(t, p.updatesOffered)
	})

	t.Run("refused trust stops before the settings prompt", func(t *testing.T) {
		// Arrange
		req := newResolveRequest(t)
		layer := credentialLayer(t, "/example.test/key")
		req.Project.Layers = []config.RCLayer{layer}
		p := &fakePrompter{trustDir: func(string) error { return errors.New("directory not trusted") }}

		// Act
		_, err := newTestService(&fakeDocker{inspectImage: built}, nil).Resolve(req, p)

		// Assert
		require.EqualError(t, err, "directory not trusted")
		assert.Empty(t, p.settingsAsked)
	})

	t.Run("--no-proxy with credentials fails after the approvals", func(t *testing.T) {
		// Arrange
		req := newResolveRequest(t)
		layer := credentialLayer(t, "/example.test/key")
		req.Project.Layers = []config.RCLayer{layer}
		req.Project.RC.Run.Proxy.Credentials = layer.RC.Run.Proxy.Credentials
		req.Flags.Proxy = resolve.ProxyInput{NoProxy: true}
		p := &fakePrompter{}

		// Act
		_, err := newTestService(&fakeDocker{inspectImage: built}, nil).Resolve(req, p)

		// Assert
		require.ErrorContains(t, err, "--no-proxy cannot be used")
		assert.Equal(t, [][]string{{config.CredentialsSetting}}, p.settingsAsked)
	})

	t.Run("approved credentials reach the run config with an injection notice", func(t *testing.T) {
		// Arrange
		stderr := iostreamstest.StubErr(t)
		req := newCredentialRequest(t)

		// Act
		plan, err := newTestService(&fakeDocker{inspectImage: built}, nil).Resolve(req, &fakePrompter{})

		// Assert
		require.NoError(t, err)
		require.Len(t, plan.config.Egress.Credentials, 1)
		assert.Equal(t, []string{"api.example.test"}, plan.config.Egress.Credentials[0].Hosts)
		assert.Contains(t, stderr.String(), "injecting credentials for api.example.test")
	})

	t.Run("dry run with approved credentials prints no injection notice", func(t *testing.T) {
		// Arrange
		stderr := iostreamstest.StubErr(t)
		req := newCredentialRequest(t)
		req.Flags.DryRun = true

		// Act
		_, err := newTestService(&fakeDocker{inspectImage: built}, nil).Resolve(req, &fakePrompter{})

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, stderr.String(), "injecting credentials")
	})
}

func Test_Service_offerToolUpdate(t *testing.T) {
	// Arrange
	req := newResolveRequest(t)
	approved := approvedRC(&config.AgenticRC{})
	var got ToolUpdate
	u := &fakeToolUpdates{offer: func(update ToolUpdate, _ func(string, string, string) bool) error {
		got = update
		return nil
	}}
	svc := newUpdatingService(&fakeDocker{}, u)

	// Act
	err := svc.offerToolUpdate(req, approved, &fakePrompter{})

	// Assert
	require.NoError(t, err)
	assert.Same(t, approved.rc, got.RC)
}

func TestCheckProjectDir(t *testing.T) {
	t.Run("network share is refused", func(t *testing.T) {
		// Act
		err := CheckProjectDir("//server.example.test/share/project")

		// Assert
		require.ErrorContains(t, err, "is on a network share")
	})

	t.Run("local dir is allowed", func(t *testing.T) {
		// Act
		err := CheckProjectDir(t.TempDir())

		// Assert
		require.NoError(t, err)
	})
}

func Test_Service_resolveRun(t *testing.T) {
	daemonCaps := sandbox.Capabilities{
		Features: sandbox.Features{
			Proxy:               true,
			Dind:                true,
			CredentialInjection: true,
		},
		Isolation: sandbox.Isolation{CredentialsViaProxy: true},
	}
	noDaemonCaps := sandbox.Capabilities{Features: sandbox.Features{CredentialInjection: true}}
	image := &docker.ImageInfo{ContainerHome: "/home/agent"}
	// newSvc returns a Service whose runtimes both offer caps, over image
	newSvc := func(caps sandbox.Capabilities) *Service {
		d := &fakeDocker{
			capabilities: func() sandbox.Capabilities { return caps },
			inspectImage: func(string) (*docker.ImageInfo, error) { return image, nil },
		}
		m := &fakeMicrosandbox{capabilities: func() sandbox.Capabilities { return caps }}
		return newTestService(d, m)
	}
	target := Target{
		ToolName:  "claude",
		ImageName: "agentic-claude",
	}

	t.Run("copies the flags and resolves proxy, dind and limits against the config", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		// No funcs, so the config compares equal
		tool := tools.ToolConfig{Runtime: tools.RuntimeConfig{WebSearchTool: "WebSearch"}}
		req := Request{
			Target: target,
			Tool:   tool,
			Flags: Flags{
				ToolHome:   "/example.test/home",
				DryRun:     true,
				Registry:   "registry.example.test",
				Proxy:      resolve.ProxyInput{MonitorFlag: true},
				Dind:       resolve.DindInput{DindFlag: true},
				Volumes:    []string{"/example.test/a:/a"},
				Env:        []string{"FOO=bar"},
				Ports:      []string{"3000"},
				Limits:     sandbox.ResourceLimits{CPUs: "2"},
				DindLimits: sandbox.ResourceLimits{Memory: "1g"},
				Runtime:    sandbox.RuntimeMicrosandbox,
			},
		}

		// Act
		c, err := newSvc(daemonCaps).resolveRun(req, rc)

		// Assert
		require.NoError(t, err)
		limits := sandbox.ResourceLimits{CPUs: "2"}.Or(sandbox.DefaultLimits)
		assert.Equal(t, runConfig{
			Target: target,
			Tool:   tool,
			RC:     rc,
			Image:  image,
			Runtime: runtimeConfig{
				Name: sandbox.RuntimeMicrosandbox,
				Caps: daemonCaps,
			},
			Homes: homes{
				Tool:      "/example.test/home",
				Container: "/home/agent",
			},
			Egress: egressConfig{Mode: sandbox.EgressMonitor},
			Limits: limits,
			Dind: sandbox.DindSpec{
				Enabled: true,
				Limits:  sandbox.ResourceLimits{Memory: "1g"}.Or(limits),
			},
			Mounts:   runMounts{Extra: []string{"/example.test/a:/a"}},
			Env:      []string{"FOO=bar"},
			Ports:    []sandbox.PortMapping{{Host: 3000, Guest: 3000}},
			Registry: "registry.example.test",
			DryRun:   true,
		}, c)
	})

	t.Run("a proxy conflict is an error", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.Credentials = []config.RCCredential{{Hosts: []string{"api.example.test"}, Header: "X-Api-Key", Secret: "/example.test/key"}}
		req := Request{
			Target: target,
			Flags:  Flags{Proxy: resolve.ProxyInput{NoProxy: true}},
		}

		// Act
		_, err := newSvc(daemonCaps).resolveRun(req, rc)

		// Assert
		require.ErrorContains(t, err, "--no-proxy cannot be used")
	})

	t.Run("an invalid port is an error", func(t *testing.T) {
		// Arrange
		req := Request{
			Target: target,
			Flags:  Flags{Ports: []string{"http"}},
		}

		// Act
		_, err := newSvc(daemonCaps).resolveRun(req, &config.AgenticRC{})

		// Assert
		require.EqualError(t, err, `port "http": "http" must be a number from 1 to 65535`)
	})

	t.Run("an image inspect error is an error", func(t *testing.T) {
		// Arrange
		svc := newTestService(&fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) { return nil, errors.New("docker daemon unreachable") },
		}, nil)

		// Act
		_, err := svc.resolveRun(Request{Target: target}, &config.AgenticRC{})

		// Assert
		require.EqualError(t, err, "inspect agentic-claude: docker daemon unreachable")
	})

	t.Run("an unknown proxy integration is an error", func(t *testing.T) {
		// Arrange
		req := Request{
			Target: target,
			Flags:  Flags{Proxy: resolve.ProxyInput{Integrations: []string{"bogus"}}},
		}

		// Act
		_, err := newSvc(daemonCaps).resolveRun(req, &config.AgenticRC{})

		// Assert
		require.ErrorContains(t, err, `--proxy-integration: unknown integration "bogus"`)
	})

	t.Run("dind stays off on a runtime without a daemon", func(t *testing.T) {
		// Arrange
		req := Request{
			Target: target,
			Flags:  Flags{Dind: resolve.DindInput{DindFlag: true}},
		}

		// Act
		c, err := newSvc(noDaemonCaps).resolveRun(req, &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.False(t, c.Dind.Enabled)
	})

	t.Run("credentials leave the proxy off when the runtime injects them", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.Credentials = []config.RCCredential{{Hosts: []string{"api.example.test"}, Header: "X-Api-Key", Secret: "/example.test/key"}}

		// Act
		c, err := newSvc(noDaemonCaps).resolveRun(Request{Target: target}, rc)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.EgressOff, c.Egress.Mode)
	})
}
