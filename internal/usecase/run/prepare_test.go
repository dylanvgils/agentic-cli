package run

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepare(t *testing.T) {
	built := func(string) (*docker.ImageInfo, error) {
		return &docker.ImageInfo{Image: "agentic-claude", Version: "1.0.0", ProxyTrust: true}, nil
	}

	t.Run("returns the run spec for the target", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)

		// Act
		rs, cleanup, err := New(&fakeDocker{inspectImage: built}).Prepare(req, &fakePrompter{})
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
		_, cleanup, err := New(&fakeDocker{}).Prepare(req, p)
		defer cleanup()

		// Assert
		require.ErrorContains(t, err, "agentic build claude")
		assert.Empty(t, p.trustAsked)
	})

	t.Run("refused trust stops before the tool update and setup", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Project.RC.Run.CheckUpdates = nil
		stubLatestToolVersion(t, "2.0.0")
		stubLog(t)
		tool := tools.Configs["claude"]
		setUp := false
		tool.Runtime.Setup = func(string) error {
			setUp = true
			return nil
		}
		req.Tool = tool
		p := &fakePrompter{trustDir: func(string) error { return errors.New("directory not trusted") }}

		// Act
		_, cleanup, err := New(&fakeDocker{inspectImage: built}).Prepare(req, p)
		defer cleanup()

		// Assert
		require.EqualError(t, err, "directory not trusted")
		assert.Empty(t, p.updatesOffered)
		assert.False(t, setUp)
	})

	t.Run("declined tool update still runs", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Project.RC.Run.CheckUpdates = nil
		stubLatestToolVersion(t, "2.0.0")
		stubLog(t)
		rebuilt := false
		d := &fakeDocker{
			inspectImage: built,
			buildTool: func(string, string, tools.BuildOptions) error {
				rebuilt = true
				return nil
			},
		}
		p := &fakePrompter{}

		// Act
		rs, cleanup, err := New(d).Prepare(req, p)
		defer cleanup()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"claude"}, p.updatesOffered)
		assert.False(t, rebuilt)
		assert.Equal(t, "agentic-claude", rs.Image)
	})

	t.Run("tool setup error is wrapped with the tool name", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		tool := tools.Configs["claude"]
		tool.Runtime.Setup = func(string) error { return errors.New("disk full") }
		req.Tool = tool

		// Act
		_, cleanup, err := New(&fakeDocker{inspectImage: built}).Prepare(req, &fakePrompter{})
		defer cleanup()

		// Assert
		require.EqualError(t, err, "setup claude: disk full")
	})

	t.Run("refused settings stop before the tool update", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Project.RC.Run.CheckUpdates = nil
		stubLatestToolVersion(t, "2.0.0")
		stubLog(t)
		rc := &config.AgenticRC{}
		rc.Run.ExtraMounts = []string{"~/.example:/x:rw"}
		req.Project.Layers = []config.RCLayer{{Path: filepath.Join(req.Project.Dir, ".agenticrc.toml"), RC: rc}}
		p := &fakePrompter{approveSettings: func(config.RCLayer, []config.GuardedSetting) error { return errors.New("settings not approved") }}

		// Act
		_, cleanup, err := New(&fakeDocker{inspectImage: built}).Prepare(req, p)
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
		_, cleanup, err := New(&fakeDocker{inspectImage: built}).Prepare(req, p)
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
		_, cleanup, err := New(&fakeDocker{inspectImage: built}).Prepare(req, p)
		defer cleanup()

		// Assert
		require.ErrorContains(t, err, "--no-proxy cannot be used")
		assert.Equal(t, [][]string{{config.CredentialsSetting}}, p.settingsAsked)
	})

	t.Run("approved credentials reach the run spec", func(t *testing.T) {
		// Arrange
		stubErrLog(t)
		req := newCredentialRequest(t)

		// Act
		rs, cleanup, err := New(&fakeDocker{inspectImage: built}).Prepare(req, &fakePrompter{})
		defer cleanup()

		// Assert
		require.NoError(t, err)
		require.Len(t, rs.Proxy.Credentials, 1)
		assert.Equal(t, []string{"api.example.test"}, rs.Proxy.Credentials[0].Hosts)
	})

	t.Run("approved credentials print the injection notice", func(t *testing.T) {
		// Arrange
		stderr := stubErrLog(t)
		req := newCredentialRequest(t)

		// Act
		_, cleanup, err := New(&fakeDocker{inspectImage: built}).Prepare(req, &fakePrompter{})
		defer cleanup()

		// Assert
		require.NoError(t, err)
		assert.Contains(t, stderr.String(), "injecting credentials for api.example.test")
	})

	t.Run("dry run with approved credentials prints no injection notice", func(t *testing.T) {
		// Arrange
		stderr := stubErrLog(t)
		req := newCredentialRequest(t)
		req.Flags.DryRun = true

		// Act
		_, cleanup, err := New(&fakeDocker{inspectImage: built}).Prepare(req, &fakePrompter{})
		defer cleanup()

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, stderr.String(), "injecting credentials")
	})

	t.Run("sidecar build error is returned", func(t *testing.T) {
		// Arrange
		stubErrLog(t)
		req := newPrepareRequest(t)
		req.Flags.Proxy = resolve.ProxyInput{MonitorFlag: true}
		d := &fakeDocker{
			inspectImage: func(name string) (*docker.ImageInfo, error) {
				if name == tools.ProxyImage {
					return nil, nil
				}
				return built(name)
			},
			buildProxyImage: func(string, string, string, tools.BuildOptions) error { return errors.New("proxy build failed") },
		}

		// Act
		_, cleanup, err := New(d).Prepare(req, &fakePrompter{})
		defer cleanup()

		// Assert
		require.EqualError(t, err, "proxy build failed")
	})

	t.Run("dind without the docker layer fails before building sidecars", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Flags.Dind = resolve.DindInput{DindFlag: true}
		var builds []string
		d := &fakeDocker{
			inspectImage:   built,
			buildDindImage: func(image string, _ tools.BuildOptions) error { builds = append(builds, image); return nil },
		}

		// Act
		_, cleanup, err := New(d).Prepare(req, &fakePrompter{})
		defer cleanup()

		// Assert
		require.ErrorContains(t, err, "--base docker")
		assert.Empty(t, builds)
	})

	t.Run("enabled sidecar images are built when missing", func(t *testing.T) {
		// Arrange
		stubErrLog(t)
		req := newPrepareRequest(t)
		req.Flags.Proxy = resolve.ProxyInput{MonitorFlag: true}
		var builds []string
		d := &fakeDocker{
			inspectImage: func(name string) (*docker.ImageInfo, error) {
				if name == tools.ProxyImage {
					return nil, nil
				}
				return built(name)
			},
			buildProxyImage: func(image, _, _ string, _ tools.BuildOptions) error { builds = append(builds, image); return nil },
		}

		// Act
		_, cleanup, err := New(d).Prepare(req, &fakePrompter{})
		defer cleanup()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{tools.ProxyImage}, builds)
	})

	t.Run("dry run builds no sidecar images", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Flags.Proxy = resolve.ProxyInput{MonitorFlag: true}
		req.Flags.DryRun = true
		var builds []string
		d := &fakeDocker{
			inspectImage: func(name string) (*docker.ImageInfo, error) {
				if name == tools.ProxyImage {
					return nil, nil
				}
				return built(name)
			},
			buildProxyImage: func(image, _, _ string, _ tools.BuildOptions) error { builds = append(builds, image); return nil },
		}

		// Act
		_, cleanup, err := New(d).Prepare(req, &fakePrompter{})
		defer cleanup()

		// Assert
		require.NoError(t, err)
		assert.Empty(t, builds)
	})
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

func Test_resolveInput(t *testing.T) {
	t.Run("copies the flags and resolves proxy and dind against the config", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		flags := Flags{
			ToolHome:       "/example.test/home",
			DryRun:         true,
			Registry:       "registry.example.test",
			Proxy:          resolve.ProxyInput{MonitorFlag: true},
			Dind:           resolve.DindInput{DindFlag: true},
			Volumes:        []string{"/example.test/a:/a"},
			Secrets:        []string{"token:/example.test/token"},
			ReadOnlyMounts: []string{"/example.test/b:/b"},
			Env:            []string{"FOO=bar"},
			Limits:         docker.ResourceLimits{CPUs: "2"},
			DindLimits:     docker.ResourceLimits{Memory: "1g"},
		}

		// Act
		in, err := resolveInput(flags, rc)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, Input{
			ToolHome:       "/example.test/home",
			Volumes:        []string{"/example.test/a:/a"},
			Secrets:        []string{"token:/example.test/token"},
			ReadOnlyMounts: []string{"/example.test/b:/b"},
			Env:            []string{"FOO=bar"},
			Limits:         docker.ResourceLimits{CPUs: "2"},
			DryRun:         true,
			Registry:       "registry.example.test",
			ProxyMode:      docker.ProxyMonitor,
			DindEnabled:    true,
			DindLimits:     docker.ResourceLimits{Memory: "1g"},
		}, in)
	})

	t.Run("a proxy conflict is an error", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{}
		rc.Run.Proxy.Credentials = []config.RCCredential{{Hosts: []string{"api.example.test"}, Header: "X-Api-Key", Secret: "/example.test/key"}}
		flags := Flags{Proxy: resolve.ProxyInput{NoProxy: true}}

		// Act
		_, err := resolveInput(flags, rc)

		// Assert
		require.ErrorContains(t, err, "--no-proxy cannot be used")
	})
}
