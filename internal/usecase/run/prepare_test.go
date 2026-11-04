package run

import (
	"errors"
	"path/filepath"
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

func TestPrepare(t *testing.T) {
	built := func(string) (*docker.ImageInfo, error) {
		return &docker.ImageInfo{Image: "agentic-claude", Version: "1.0.0", ProxyTrust: true}, nil
	}

	// dueUpdate offers an update from 1.0.0 to 2.0.0 through the prompter, as a due update check does
	dueUpdate := &fakeToolUpdates{offer: func(u ToolUpdate, confirm func(tool, installed, latest string) bool) error {
		confirm(u.Tool, "1.0.0", "2.0.0")
		return nil
	}}

	t.Run("returns the run spec for the target", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)

		// Act
		rs, cleanup, err := newTestService(&fakeDocker{inspectImage: built}, nil).Prepare(req, &fakePrompter{})
		defer cleanup()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude", rs.Image)
	})

	t.Run("missing image stops before any prompt", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		p := &fakePrompter{}

		// Act
		_, cleanup, err := newTestService(&fakeDocker{}, nil).Prepare(req, p)
		defer cleanup()

		// Assert
		require.ErrorContains(t, err, "agentic build claude")
		assert.Empty(t, p.trustAsked)
	})

	t.Run("missing msb stops before the image check and any prompt", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Flags.Runtime = sandbox.RuntimeMicrosandbox
		inspected := false
		d := &fakeDocker{inspectImage: func(name string) (*docker.ImageInfo, error) {
			inspected = true
			return built(name)
		}}
		p := &fakePrompter{}
		m := &fakeMicrosandbox{available: func() error { return errors.New("msb not found on PATH") }}

		// Act
		_, cleanup, err := newTestService(d, m).Prepare(req, p)
		defer cleanup()

		// Assert
		require.EqualError(t, err, "msb not found on PATH")
		assert.False(t, inspected)
		assert.Empty(t, p.trustAsked)
	})

	t.Run("dind on an image without the docker layer stops before any prompt", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Flags.Dind.DindFlag = true
		p := &fakePrompter{}

		// Act
		_, cleanup, err := newTestService(&fakeDocker{inspectImage: built}, nil).Prepare(req, p)
		defer cleanup()

		// Assert
		require.ErrorContains(t, err, "--base docker")
		assert.Empty(t, p.trustAsked)
	})

	t.Run("a credential without env stops before any prompt", func(t *testing.T) {
		// Arrange
		req := newCredentialRequest(t)
		req.Project.RC.Run.Proxy.Credentials[0].Env = nil
		p := &fakePrompter{}

		// Act
		_, cleanup, err := newTestService(&fakeDocker{inspectImage: built}, nil).Prepare(req, p)
		defer cleanup()

		// Assert
		require.ErrorContains(t, err, "needs env, as the tool must send the placeholder")
		assert.Empty(t, p.trustAsked)
	})

	t.Run("microsandbox in proxy monitor mode stops before any prompt", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Flags.Runtime = sandbox.RuntimeMicrosandbox
		req.Flags.Proxy.MonitorFlag = true
		p := &fakePrompter{}

		// Act
		_, cleanup, err := newTestService(&fakeDocker{inspectImage: built}, &fakeMicrosandbox{}).Prepare(req, p)
		defer cleanup()

		// Assert
		require.ErrorContains(t, err, "does not support proxy monitor mode (--proxy-monitor)")
		assert.Empty(t, p.trustAsked)
	})

	t.Run("docker runtime doesn't check for msb", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		m := &fakeMicrosandbox{available: func() error { return errors.New("msb not found on PATH") }}

		// Act
		_, cleanup, err := newTestService(&fakeDocker{inspectImage: built}, m).Prepare(req, &fakePrompter{})
		defer cleanup()

		// Assert
		assert.NoError(t, err)
	})

	t.Run("microsandbox dry run doesn't check for msb", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Flags.Runtime = sandbox.RuntimeMicrosandbox
		req.Flags.DryRun = true
		m := &fakeMicrosandbox{available: func() error { return errors.New("msb not found on PATH") }}

		// Act
		_, cleanup, err := newTestService(&fakeDocker{inspectImage: built}, m).Prepare(req, &fakePrompter{})
		defer cleanup()

		// Assert
		assert.NoError(t, err)
	})

	t.Run("refused trust stops before the tool update and setup", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
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
		_, cleanup, err := New(&fakeDocker{inspectImage: built}, nil, dueUpdate, &fakeSidecarImages{}).Prepare(req, p)
		defer cleanup()

		// Assert
		require.EqualError(t, err, "directory not trusted")
		assert.Empty(t, p.updatesOffered)
		assert.False(t, setUp)
	})

	t.Run("offers the tool update through the prompter", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		iostreamstest.StubOut(t)
		var got ToolUpdate
		u := &fakeToolUpdates{offer: func(update ToolUpdate, confirm func(tool, installed, latest string) bool) error {
			got = update
			confirm(update.Tool, "1.0.0", "2.0.0")
			return nil
		}}
		p := &fakePrompter{}

		// Act
		_, cleanup, err := New(&fakeDocker{inspectImage: built}, nil, u, &fakeSidecarImages{}).Prepare(req, p)
		defer cleanup()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, ToolUpdate{Home: req.Flags.ToolHome, RC: req.Project.RC, Tool: "claude", Image: "agentic-claude"}, got)
		assert.Equal(t, []string{"claude"}, p.updatesOffered)
	})

	t.Run("a failed tool update stops the run", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		iostreamstest.StubOut(t)
		u := &fakeToolUpdates{offer: func(ToolUpdate, func(string, string, string) bool) error { return errors.New("update failed") }}

		// Act
		_, cleanup, err := New(&fakeDocker{inspectImage: built}, nil, u, &fakeSidecarImages{}).Prepare(req, &fakePrompter{})
		defer cleanup()

		// Assert
		require.EqualError(t, err, "update failed")
	})

	t.Run("dry run offers no tool update", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Flags.DryRun = true
		iostreamstest.StubOut(t)
		p := &fakePrompter{}

		// Act
		_, cleanup, err := New(&fakeDocker{inspectImage: built}, nil, dueUpdate, &fakeSidecarImages{}).Prepare(req, p)
		defer cleanup()

		// Assert
		require.NoError(t, err)
		assert.Empty(t, p.updatesOffered)
	})

	t.Run("tool setup error is wrapped with the tool name", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		tool := tools.Configs["claude"]
		tool.Runtime.Setup = func(string) error { return errors.New("disk full") }
		req.Tool = tool

		// Act
		_, cleanup, err := newTestService(&fakeDocker{inspectImage: built}, nil).Prepare(req, &fakePrompter{})
		defer cleanup()

		// Assert
		require.EqualError(t, err, "setup claude: disk full")
	})

	t.Run("refused settings stop before the tool update", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		iostreamstest.StubOut(t)
		rc := &config.AgenticRC{}
		rc.Run.Mounts = []string{"~/.example:/x:rw"}
		req.Project.Layers = []config.RCLayer{{Path: filepath.Join(req.Project.Dir, ".agenticrc.toml"), RC: rc}}
		p := &fakePrompter{approveSettings: func(config.RCLayer, []config.GuardedSetting) error { return errors.New("settings not approved") }}

		// Act
		_, cleanup, err := New(&fakeDocker{inspectImage: built}, nil, dueUpdate, &fakeSidecarImages{}).Prepare(req, p)
		defer cleanup()

		// Assert
		require.EqualError(t, err, "settings not approved")
		assert.Empty(t, p.updatesOffered)
	})

	t.Run("refused trust stops before the settings prompt", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		layer := credentialLayer(t, "/example.test/key")
		req.Project.Layers = []config.RCLayer{layer}
		p := &fakePrompter{trustDir: func(string) error { return errors.New("directory not trusted") }}

		// Act
		_, cleanup, err := newTestService(&fakeDocker{inspectImage: built}, nil).Prepare(req, p)
		defer cleanup()

		// Assert
		require.EqualError(t, err, "directory not trusted")
		assert.Empty(t, p.settingsAsked)
	})

	t.Run("--no-proxy with credentials fails after the approvals", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		layer := credentialLayer(t, "/example.test/key")
		req.Project.Layers = []config.RCLayer{layer}
		req.Project.RC.Run.Proxy.Credentials = layer.RC.Run.Proxy.Credentials
		req.Flags.Proxy = resolve.ProxyInput{NoProxy: true}
		p := &fakePrompter{}

		// Act
		_, cleanup, err := newTestService(&fakeDocker{inspectImage: built}, nil).Prepare(req, p)
		defer cleanup()

		// Assert
		require.ErrorContains(t, err, "--no-proxy cannot be used")
		assert.Equal(t, [][]string{{config.CredentialsSetting}}, p.settingsAsked)
	})

	t.Run("approved credentials reach the run spec", func(t *testing.T) {
		// Arrange
		iostreamstest.StubErr(t)
		req := newCredentialRequest(t)

		// Act
		rs, cleanup, err := newTestService(&fakeDocker{inspectImage: built}, nil).Prepare(req, &fakePrompter{})
		defer cleanup()

		// Assert
		require.NoError(t, err)
		require.Len(t, rs.Credentials, 1)
		assert.Equal(t, []string{"api.example.test"}, rs.Credentials[0].Hosts)
	})

	t.Run("approved credentials print the injection notice", func(t *testing.T) {
		// Arrange
		stderr := iostreamstest.StubErr(t)
		req := newCredentialRequest(t)

		// Act
		_, cleanup, err := newTestService(&fakeDocker{inspectImage: built}, nil).Prepare(req, &fakePrompter{})
		defer cleanup()

		// Assert
		require.NoError(t, err)
		assert.Contains(t, stderr.String(), "injecting credentials for api.example.test")
	})

	t.Run("dry run with approved credentials prints no injection notice", func(t *testing.T) {
		// Arrange
		stderr := iostreamstest.StubErr(t)
		req := newCredentialRequest(t)
		req.Flags.DryRun = true

		// Act
		_, cleanup, err := newTestService(&fakeDocker{inspectImage: built}, nil).Prepare(req, &fakePrompter{})
		defer cleanup()

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, stderr.String(), "injecting credentials")
	})

}

func Test_Service_offerToolUpdate(t *testing.T) {
	// Arrange
	req := newPrepareRequest(t)
	approved := approvedRC(&config.AgenticRC{})
	var got ToolUpdate
	u := &fakeToolUpdates{offer: func(update ToolUpdate, _ func(string, string, string) bool) error {
		got = update
		return nil
	}}
	svc := New(&fakeDocker{}, nil, u, &fakeSidecarImages{})

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

func Test_Request_runtime(t *testing.T) {
	// Arrange
	req := Request{Project: Project{RC: &config.AgenticRC{Run: config.RCRun{Runtime: "microsandbox"}}}}

	// Act
	runtime := req.runtime()

	// Assert
	assert.Equal(t, sandbox.RuntimeMicrosandbox, runtime)
}

func Test_resolveInput(t *testing.T) {
	dockerCaps := sandbox.Capabilities{
		Features: sandbox.Features{
			Proxy:               true,
			Dind:                true,
			CredentialInjection: true,
		},
		Isolation: sandbox.Isolation{CredentialsViaProxy: true},
	}

	t.Run("copies the flags and resolves proxy and dind against the config", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		flags := Flags{
			ToolHome:   "/example.test/home",
			DryRun:     true,
			Registry:   "registry.example.test",
			Proxy:      resolve.ProxyInput{MonitorFlag: true},
			Dind:       resolve.DindInput{DindFlag: true},
			Volumes:    []string{"/example.test/a:/a"},
			Env:        []string{"FOO=bar"},
			Limits:     sandbox.ResourceLimits{CPUs: "2"},
			DindLimits: sandbox.ResourceLimits{Memory: "1g"},
			Runtime:    sandbox.RuntimeMicrosandbox,
		}

		// Act
		in, err := resolveInput(flags, rc, dockerCaps)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, Input{
			ToolHome:    "/example.test/home",
			Volumes:     []string{"/example.test/a:/a"},
			Env:         []string{"FOO=bar"},
			Limits:      sandbox.ResourceLimits{CPUs: "2"},
			DryRun:      true,
			Registry:    "registry.example.test",
			ProxyMode:   sandbox.EgressMonitor,
			DindEnabled: true,
			DindLimits:  sandbox.ResourceLimits{Memory: "1g"},
			Runtime:     sandbox.RuntimeMicrosandbox,
		}, in)
	})

	t.Run("config picks the runtime when the flag is unset", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Runtime: "microsandbox"}}

		// Act
		in, err := resolveInput(Flags{}, rc, dockerCaps)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.RuntimeMicrosandbox, in.Runtime)
	})

	t.Run("a proxy conflict is an error", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.Credentials = []config.RCCredential{{Hosts: []string{"api.example.test"}, Header: "X-Api-Key", Secret: "/example.test/key"}}
		flags := Flags{Proxy: resolve.ProxyInput{NoProxy: true}}

		// Act
		_, err := resolveInput(flags, rc, dockerCaps)

		// Assert
		require.ErrorContains(t, err, "--no-proxy cannot be used")
	})

	t.Run("an unknown proxy integration is an error", func(t *testing.T) {
		// Arrange
		flags := Flags{Proxy: resolve.ProxyInput{Integrations: []string{"bogus"}}}

		// Act
		_, err := resolveInput(flags, &config.AgenticRC{}, dockerCaps)

		// Assert
		require.ErrorContains(t, err, `--proxy-integration: unknown integration "bogus"`)
	})

	t.Run("dind stays off on a runtime without a daemon", func(t *testing.T) {
		// Arrange
		flags := Flags{Dind: resolve.DindInput{DindFlag: true}}

		// Act
		in, err := resolveInput(flags, &config.AgenticRC{}, sandbox.Capabilities{Features: sandbox.Features{CredentialInjection: true}})

		// Assert
		require.NoError(t, err)
		assert.False(t, in.DindEnabled)
	})

	t.Run("credentials leave the proxy off when the runtime injects them", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.Credentials = []config.RCCredential{{Hosts: []string{"api.example.test"}, Header: "X-Api-Key", Secret: "/example.test/key"}}

		// Act
		in, err := resolveInput(Flags{}, rc, sandbox.Capabilities{Features: sandbox.Features{CredentialInjection: true}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.EgressOff, in.ProxyMode)
	})
}
