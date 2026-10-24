package toolruns

import (
	"errors"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Service_List(t *testing.T) {
	d := &fakeDocker{listRunningContainers: func() ([]*docker.ContainerInfo, error) {
		return []*docker.ContainerInfo{
			{
				Name:      "agentic-claude-ab12",
				Image:     "agentic-claude",
				Namespace: "agentic",
				Tool:      "claude",
				Status:    "Up 5 minutes",
			},
			{
				Name:      "other-copilot-cd34",
				Image:     "other-copilot",
				Namespace: "other",
				Tool:      "copilot",
				Status:    "Up 1 minute",
			},
			{
				// A sidecar, which has no tool
				Name:   "agentic-proxy-ab12",
				Status: "Up 5 minutes",
			},
		}, nil
	}}
	msb := &fakeMicrosandbox{listRunning: func() ([]*microsandbox.Sandbox, error) {
		return []*microsandbox.Sandbox{
			{
				Name:      "vm-1",
				Image:     "agentic-opencode:latest",
				Namespace: "agentic",
				Tool:      "opencode",
				Status:    "running",
			},
		}, nil
	}}
	claude := Run{
		Tool:      "claude",
		Namespace: "agentic",
		Runtime:   sandbox.RuntimeDocker,
		Image:     "agentic-claude",
		Status:    "Up 5 minutes",
		Name:      "agentic-claude-ab12",
	}
	copilot := Run{
		Tool:      "copilot",
		Namespace: "other",
		Runtime:   sandbox.RuntimeDocker,
		Image:     "other-copilot",
		Status:    "Up 1 minute",
		Name:      "other-copilot-cd34",
	}
	opencode := Run{
		Tool:      "opencode",
		Namespace: "agentic",
		Runtime:   sandbox.RuntimeMicrosandbox,
		Image:     "agentic-opencode:latest",
		Status:    "running",
		Name:      "vm-1",
	}

	t.Run("joins every runtime's runs in the namespace without sidecars", func(t *testing.T) {
		// Arrange
		svc := New(d, msb)

		// Act
		runs := svc.List("agentic")

		// Assert
		assert.Equal(t, []Run{claude, opencode}, runs)
	})

	t.Run("returns every namespace's runs for an empty namespace", func(t *testing.T) {
		// Arrange
		svc := New(d, msb)

		// Act
		runs := svc.List("")

		// Assert
		assert.Equal(t, []Run{claude, copilot, opencode}, runs)
	})
}

func Test_Service_Images(t *testing.T) {
	t.Run("keeps tool images and leaves out sidecar images", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Tool: "claude"},
				{Image: "agentic-proxy", Tool: "proxy"},
			}, nil
		}}
		svc := New(d, &fakeMicrosandbox{})

		// Act
		images, err := svc.Images("")

		// Assert
		require.NoError(t, err)
		require.Len(t, images, 1)
		assert.Equal(t, "agentic-claude", images[0].Image)
	})

	t.Run("namespace narrows the listing", func(t *testing.T) {
		// Arrange
		var filters []docker.ImageFilter
		d := &fakeDocker{listAllImages: func(f ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			filters = f
			return nil, nil
		}}
		svc := New(d, &fakeMicrosandbox{})

		// Act
		_, err := svc.Images("work")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []docker.ImageFilter{docker.NamespaceFilter("work")}, filters)
	})

	t.Run("empty namespace lists every namespace", func(t *testing.T) {
		// Arrange
		var filters []docker.ImageFilter
		d := &fakeDocker{listAllImages: func(f ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			filters = f
			return nil, nil
		}}
		svc := New(d, &fakeMicrosandbox{})

		// Act
		_, err := svc.Images("")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, filters)
	})

	t.Run("docker error is returned", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, errors.New("docker daemon not running")
		}}
		svc := New(d, &fakeMicrosandbox{})

		// Act
		_, err := svc.Images("")

		// Assert
		require.ErrorContains(t, err, "docker daemon not running")
	})
}

func Test_Service_dockerRuns(t *testing.T) {
	t.Run("returns each container as a Docker run", func(t *testing.T) {
		// Arrange
		svc := New(&fakeDocker{listRunningContainers: func() ([]*docker.ContainerInfo, error) {
			return []*docker.ContainerInfo{
				{
					Name:      "agentic-claude-ab12",
					Image:     "agentic-claude",
					Namespace: "agentic",
					Tool:      "claude",
					Status:    "Up 5 minutes",
				},
			}, nil
		}}, &fakeMicrosandbox{})

		// Act
		runs := svc.dockerRuns()

		// Assert
		assert.Equal(t, []Run{
			{
				Tool:      "claude",
				Namespace: "agentic",
				Runtime:   sandbox.RuntimeDocker,
				Image:     "agentic-claude",
				Status:    "Up 5 minutes",
				Name:      "agentic-claude-ab12",
			},
		}, runs)
	})

	t.Run("warns and returns nothing when the daemon is down", func(t *testing.T) {
		// Arrange
		logs := stubErrLog(t)
		svc := New(&fakeDocker{
			checkDaemon:           func() error { return docker.ErrDaemonNotRunning },
			listRunningContainers: func() ([]*docker.ContainerInfo, error) { return nil, errors.New("unexpected docker call") },
		}, &fakeMicrosandbox{})

		// Act
		runs := svc.dockerRuns()

		// Assert
		assert.Empty(t, runs)
		assert.Contains(t, logs.String(), "skipping Docker")
	})

	t.Run("warns and returns nothing when listing fails", func(t *testing.T) {
		// Arrange
		logs := stubErrLog(t)
		svc := New(&fakeDocker{listRunningContainers: func() ([]*docker.ContainerInfo, error) {
			return nil, errors.New("boom")
		}}, &fakeMicrosandbox{})

		// Act
		runs := svc.dockerRuns()

		// Assert
		assert.Empty(t, runs)
		assert.Contains(t, logs.String(), "list Docker containers: boom")
	})
}

func Test_Service_msbRuns(t *testing.T) {
	t.Run("returns each sandbox as a microsandbox run", func(t *testing.T) {
		// Arrange
		svc := New(&fakeDocker{}, &fakeMicrosandbox{listRunning: func() ([]*microsandbox.Sandbox, error) {
			return []*microsandbox.Sandbox{
				{
					Name:      "vm-1",
					Image:     "agentic-opencode:latest",
					Namespace: "agentic",
					Tool:      "opencode",
					Status:    "running",
				},
			}, nil
		}})

		// Act
		runs := svc.msbRuns()

		// Assert
		assert.Equal(t, []Run{
			{
				Tool:      "opencode",
				Namespace: "agentic",
				Runtime:   sandbox.RuntimeMicrosandbox,
				Image:     "agentic-opencode:latest",
				Status:    "running",
				Name:      "vm-1",
			},
		}, runs)
	})

	t.Run("returns nothing when msb isn't installed", func(t *testing.T) {
		// Arrange
		svc := New(&fakeDocker{}, &fakeMicrosandbox{
			available:   func() error { return errors.New("msb not found") },
			listRunning: func() ([]*microsandbox.Sandbox, error) { return nil, errors.New("unexpected msb call") },
		})

		// Act
		runs := svc.msbRuns()

		// Assert
		assert.Empty(t, runs)
	})

	t.Run("warns and returns nothing when listing fails", func(t *testing.T) {
		// Arrange
		logs := stubErrLog(t)
		svc := New(&fakeDocker{}, &fakeMicrosandbox{listRunning: func() ([]*microsandbox.Sandbox, error) {
			return nil, errors.New("list msb sandboxes: boom")
		}})

		// Act
		runs := svc.msbRuns()

		// Assert
		assert.Empty(t, runs)
		assert.Contains(t, logs.String(), "list msb sandboxes: boom")
	})
}
