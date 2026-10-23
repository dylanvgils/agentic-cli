package cli

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runStatus(t *testing.T) {
	stubMicrosandbox(t, &fakeMicrosandbox{})

	t.Run("daemon not running reports status without listing containers", func(t *testing.T) {
		// Arrange
		cmd, buf := newTestStatusCmd(t)
		stubCurrentContext(t, func() (string, error) { return "desktop-linux", nil })
		stubCheckDockerDaemon(t, func() error { return docker.ErrDaemonNotRunning })
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) {
			t.Fatal("listRunningContainers should not be called when the daemon is down")
			return nil, nil
		})

		// Act
		err := runStatus(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "Docker:          not running\nDocker context:  desktop-linux\nMicrosandbox:    installed\nVMs running:     0\n", buf.String())
	})

	t.Run("daemon running with no containers prints zero count", func(t *testing.T) {
		// Arrange
		cmd, buf := newTestStatusCmd(t)
		stubCurrentContext(t, func() (string, error) { return "desktop-linux", nil })
		stubCheckDockerDaemon(t, func() error { return nil })
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) { return nil, nil })

		// Act
		err := runStatus(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "Docker:              running\nDocker context:      desktop-linux\nMicrosandbox:        installed\nContainers running:  0\nVMs running:         0\n", buf.String())
	})

	t.Run("msb installed prints its VM count", func(t *testing.T) {
		// Arrange
		cmd, buf := newTestStatusCmd(t)
		stubCheckDockerDaemon(t, func() error { return nil })
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) {
			return []*docker.ContainerInfo{{Name: "agentic-claude-ab12", Tool: "claude"}}, nil
		})
		stubMicrosandbox(t, &fakeMicrosandbox{listRunning: func() ([]*microsandbox.Sandbox, error) {
			return []*microsandbox.Sandbox{{Name: "vm-1", Tool: "claude"}, {Name: "vm-2", Tool: "copilot"}}, nil
		}})

		// Act
		err := runStatus(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Containers running:  1\n")
		assert.Contains(t, buf.String(), "VMs running:         2\n")
		assert.NotContains(t, buf.String(), "agentic-claude-ab12")
	})

	t.Run("msb list error leaves out the VM count", func(t *testing.T) {
		// Arrange
		cmd, buf := newTestStatusCmd(t)
		stubCheckDockerDaemon(t, func() error { return nil })
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) { return nil, nil })
		stubMicrosandbox(t, &fakeMicrosandbox{listRunning: func() ([]*microsandbox.Sandbox, error) {
			return nil, fmt.Errorf("msb error")
		}})

		// Act
		err := runStatus(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.NotContains(t, buf.String(), "VMs running")
	})

	t.Run("msb not installed leaves out the VM count", func(t *testing.T) {
		// Arrange
		cmd, buf := newTestStatusCmd(t)
		stubCheckDockerDaemon(t, func() error { return nil })
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) { return nil, nil })
		stubMicrosandbox(t, &fakeMicrosandbox{available: func() error { return fmt.Errorf("msb not found") }})

		// Act
		err := runStatus(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Microsandbox:        not installed\n")
		assert.NotContains(t, buf.String(), "VMs running")
	})

	t.Run("listRunningContainers error propagates", func(t *testing.T) {
		// Arrange
		cmd, _ := newTestStatusCmd(t)
		stubCheckDockerDaemon(t, func() error { return nil })
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) {
			return nil, fmt.Errorf("docker error")
		})

		// Act
		err := runStatus(cmd, nil)

		// Assert
		require.Error(t, err)
	})

	t.Run("context lookup error prints dash", func(t *testing.T) {
		// Arrange
		cmd, buf := newTestStatusCmd(t)
		stubCurrentContext(t, func() (string, error) { return "", fmt.Errorf("docker error") })
		stubCheckDockerDaemon(t, func() error { return nil })
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) { return nil, nil })

		// Act
		err := runStatus(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Docker context:      -\n")
	})

	t.Run("explicit docker context prints under daemon status", func(t *testing.T) {
		// Arrange
		cmd, buf := newTestStatusCmd(t)
		stubDocker(t, &fakeDocker{context: "prod"})
		stubCheckDockerDaemon(t, func() error { return nil })
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) { return nil, nil })

		// Act
		err := runStatus(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "Docker:              running\nDocker context:      prod\nMicrosandbox:        installed\nContainers running:  0\nVMs running:         0\n", buf.String())
	})
}

func Test_msbStatus(t *testing.T) {
	t.Run("installed", func(t *testing.T) {
		// Arrange
		stubMicrosandbox(t, &fakeMicrosandbox{})

		// Act
		status := msbStatus()

		// Assert
		assert.Equal(t, "installed", status)
	})

	t.Run("not installed", func(t *testing.T) {
		// Arrange
		stubMicrosandbox(t, &fakeMicrosandbox{available: func() error { return fmt.Errorf("msb not found") }})

		// Act
		status := msbStatus()

		// Assert
		assert.Equal(t, "not installed", status)
	})
}
