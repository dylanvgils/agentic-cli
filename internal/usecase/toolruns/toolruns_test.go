package toolruns

import (
	"errors"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
)

func Test_Service_List(t *testing.T) {
	d := &fakeDocker{listRunningContainers: func() ([]*docker.ContainerInfo, error) {
		return []*docker.ContainerInfo{
			{
				Name:      "agentic-claude-ab12",
				Namespace: "agentic",
				Tool:      "claude",
				Status:    "Up 5 minutes",
			},
			{
				Name:      "other-copilot-cd34",
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
		Status:    "Up 5 minutes",
		Name:      "agentic-claude-ab12",
	}
	copilot := Run{
		Tool:      "copilot",
		Namespace: "other",
		Runtime:   sandbox.RuntimeDocker,
		Status:    "Up 1 minute",
		Name:      "other-copilot-cd34",
	}
	opencode := Run{
		Tool:      "opencode",
		Namespace: "agentic",
		Runtime:   sandbox.RuntimeMicrosandbox,
		Status:    "running",
		Name:      "vm-1",
	}

	t.Run("returns the namespace's runs on every runtime without sidecars", func(t *testing.T) {
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

	t.Run("skips Docker with a warning when the daemon is down", func(t *testing.T) {
		// Arrange
		logs := stubErrLog(t)
		svc := New(&fakeDocker{checkDaemon: func() error { return docker.ErrDaemonNotRunning }}, msb)

		// Act
		runs := svc.List("agentic")

		// Assert
		assert.Equal(t, []Run{opencode}, runs)
		assert.Contains(t, logs.String(), "skipping Docker")
	})

	t.Run("skips msb with a warning when listing fails", func(t *testing.T) {
		// Arrange
		logs := stubErrLog(t)
		svc := New(d, &fakeMicrosandbox{listRunning: func() ([]*microsandbox.Sandbox, error) {
			return nil, errors.New("list msb sandboxes: boom")
		}})

		// Act
		runs := svc.List("agentic")

		// Assert
		assert.Equal(t, []Run{claude}, runs)
		assert.Contains(t, logs.String(), "list msb sandboxes: boom")
	})

	t.Run("skips msb when it isn't installed", func(t *testing.T) {
		// Arrange
		svc := New(d, &fakeMicrosandbox{
			available:   func() error { return errors.New("msb not found") },
			listRunning: func() ([]*microsandbox.Sandbox, error) { return nil, errors.New("unexpected msb call") },
		})

		// Act
		runs := svc.List("agentic")

		// Assert
		assert.Equal(t, []Run{claude}, runs)
	})
}
