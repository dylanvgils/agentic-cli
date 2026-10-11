package run

import (
	"errors"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Service_runDocker(t *testing.T) {
	req := Request{Target: Target{ToolName: "claude"}}

	t.Run("readies the host before running", func(t *testing.T) {
		// Arrange
		var calls []string
		d := &fakeDocker{
			ensureNamedVolumes: func([]string, string, string, string) error {
				calls = append(calls, "volumes")
				return nil
			},
			runContainer: func(docker.RunSpec, []string) error {
				calls = append(calls, "run")
				return nil
			},
		}

		// Act
		err := New(d, nil).runDocker(req, docker.RunSpec{Image: "agentic-claude"}, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"volumes", "run"}, calls)
	})

	t.Run("a host setup error stops the run", func(t *testing.T) {
		// Arrange
		ran := false
		d := &fakeDocker{
			ensureNamedVolumes: func([]string, string, string, string) error { return errors.New("volume error") },
			runContainer: func(docker.RunSpec, []string) error {
				ran = true
				return nil
			},
		}

		// Act
		err := New(d, nil).runDocker(req, docker.RunSpec{Image: "agentic-claude"}, nil)

		// Assert
		assert.EqualError(t, err, "volume error")
		assert.False(t, ran)
	})

	t.Run("dry run skips the host setup", func(t *testing.T) {
		// Arrange
		setUp, ran := false, false
		d := &fakeDocker{
			ensureNamedVolumes: func([]string, string, string, string) error {
				setUp = true
				return nil
			},
			runContainer: func(docker.RunSpec, []string) error {
				ran = true
				return nil
			},
		}

		// Act
		err := New(d, nil).runDocker(req, docker.RunSpec{Image: "agentic-claude", DryRun: true}, nil)

		// Assert
		require.NoError(t, err)
		assert.False(t, setUp)
		assert.True(t, ran)
	})

	t.Run("dind without the docker layer fails before building sidecars", func(t *testing.T) {
		// Arrange
		var builds []string
		d := &fakeDocker{
			inspectImage: func(name string) (*docker.ImageInfo, error) {
				return &docker.ImageInfo{Image: name}, nil
			},
			buildDindImage: func(image string, _ tools.BuildOptions) error {
				builds = append(builds, image)
				return nil
			},
		}
		rs := docker.RunSpec{Image: "agentic-claude", Dind: docker.DindSpec{Enabled: true}}

		// Act
		err := New(d, nil).runDocker(req, rs, nil)

		// Assert
		require.ErrorContains(t, err, "--base docker")
		assert.Empty(t, builds)
	})
}

func Test_Service_prepareDockerHost(t *testing.T) {
	// The proxy image is missing, so Ensure builds it
	proxyMissing := func(name string) (*docker.ImageInfo, error) {
		if name == tools.ProxyImage {
			return nil, nil
		}
		return &docker.ImageInfo{Image: name}, nil
	}

	t.Run("creates the named volumes with the registry's chown image", func(t *testing.T) {
		// Arrange
		var gotVolumes []string
		var gotChownImage string
		d := &fakeDocker{ensureNamedVolumes: func(volumes []string, _, _, chownImage string) error {
			gotVolumes, gotChownImage = volumes, chownImage
			return nil
		}}
		rs := docker.RunSpec{Volumes: []string{"maven:/cache"}}

		// Act
		err := New(d, nil).prepareDockerHost(rs, "registry.example.test")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"maven:/cache"}, gotVolumes)
		assert.Equal(t, tools.BusyboxImageFor("registry.example.test"), gotChownImage)
	})

	t.Run("builds missing sidecar images", func(t *testing.T) {
		// Arrange
		stubErrLog(t)
		var builds []string
		d := &fakeDocker{
			inspectImage: proxyMissing,
			buildProxyImage: func(image, _, _ string, _ tools.BuildOptions) error {
				builds = append(builds, image)
				return nil
			},
		}
		rs := docker.RunSpec{Proxy: docker.ProxySpec{Mode: docker.ProxyMonitor}}

		// Act
		err := New(d, nil).prepareDockerHost(rs, "")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{tools.ProxyImage}, builds)
	})

	t.Run("sidecar build error is returned", func(t *testing.T) {
		// Arrange
		stubErrLog(t)
		d := &fakeDocker{
			inspectImage:    proxyMissing,
			buildProxyImage: func(string, string, string, tools.BuildOptions) error { return errors.New("proxy build failed") },
		}
		rs := docker.RunSpec{Proxy: docker.ProxySpec{Mode: docker.ProxyMonitor}}

		// Act
		err := New(d, nil).prepareDockerHost(rs, "")

		// Assert
		assert.EqualError(t, err, "proxy build failed")
	})

	t.Run("ensures agentic-net without sidecars", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{ensureNetwork: func() error { return errors.New("network error") }}

		// Act
		err := New(d, nil).prepareDockerHost(docker.RunSpec{}, "")

		// Assert
		assert.EqualError(t, err, "network error")
	})

	t.Run("skips agentic-net with a sidecar, which joins a per-run network", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage:  func(name string) (*docker.ImageInfo, error) { return &docker.ImageInfo{Image: name}, nil },
			ensureNetwork: func() error { return errors.New("network error") },
		}
		rs := docker.RunSpec{Proxy: docker.ProxySpec{Mode: docker.ProxyEnforce}}

		// Act
		err := New(d, nil).prepareDockerHost(rs, "")

		// Assert
		assert.NoError(t, err)
	})
}
