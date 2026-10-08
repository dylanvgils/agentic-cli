package run

import (
	"errors"
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

	t.Run("declined tool update still runs", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.RC.Run.CheckUpdates = nil
		stubLatestToolVersion(t, "2.0.0")
		applied := false
		req.ApplyUpdate = func(string, string) error {
			applied = true
			return nil
		}
		p := &fakePrompter{}

		// Act
		rs, cleanup, err := New(&fakeDocker{inspectImage: built}).Prepare(req, p)
		defer cleanup()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"claude"}, p.updatesOffered)
		assert.False(t, applied)
		assert.Equal(t, "agentic-claude", rs.Image)
	})

	t.Run("failed tool update stops before the trust prompt", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.RC.Run.CheckUpdates = nil
		stubLatestToolVersion(t, "2.0.0")
		req.ApplyUpdate = func(string, string) error { return errors.New("build failed") }
		p := &fakePrompter{offerToolUpdate: func(string, string, string) bool { return true }}

		// Act
		_, cleanup, err := New(&fakeDocker{inspectImage: built}).Prepare(req, p)
		defer cleanup()

		// Assert
		require.ErrorContains(t, err, "build failed")
		assert.Empty(t, p.trustAsked)
	})

	t.Run("refused trust stops before the credentials prompt", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		layer := credentialLayer(t, "/example.test/key")
		req.Layers = []config.RCLayer{layer}
		p := &fakePrompter{trustDir: func(string) error { return errors.New("directory not trusted") }}

		// Act
		_, cleanup, err := New(&fakeDocker{inspectImage: built}).Prepare(req, p)
		defer cleanup()

		// Assert
		require.EqualError(t, err, "directory not trusted")
		assert.Empty(t, p.credentialsAsked)
	})

	t.Run("--no-proxy with credentials fails after the approvals", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		layer := credentialLayer(t, "/example.test/key")
		req.Layers = []config.RCLayer{layer}
		req.RC.Run.Proxy.Credentials = layer.RC.Run.Proxy.Credentials
		req.Proxy = resolve.ProxyInput{NoProxy: true}
		p := &fakePrompter{}

		// Act
		_, cleanup, err := New(&fakeDocker{inspectImage: built}).Prepare(req, p)
		defer cleanup()

		// Assert
		require.ErrorContains(t, err, "--no-proxy cannot be used")
		assert.Equal(t, []string{layer.Path}, p.credentialsAsked)
	})

	t.Run("dind without the docker layer fails before building sidecars", func(t *testing.T) {
		// Arrange
		req := newPrepareRequest(t)
		req.Input.DindEnabled = true
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
		req.Proxy = resolve.ProxyInput{MonitorFlag: true}
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
		req.Proxy = resolve.ProxyInput{MonitorFlag: true}
		req.Input.DryRun = true
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
