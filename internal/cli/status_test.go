package cli

import (
	"cmp"
	"errors"
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runStatus(t *testing.T) {
	noContainers := func() ([]*docker.ContainerInfo, error) { return nil, nil }

	for _, tc := range []struct {
		name   string
		docker *fakeDocker
		msb    *fakeMicrosandbox
		want   string
	}{
		{
			name: "daemon not running reports status without listing containers",
			docker: &fakeDocker{
				context:     "desktop-linux",
				checkDaemon: func() error { return docker.ErrDaemonNotRunning },
				listRunningContainers: func() ([]*docker.ContainerInfo, error) {
					return nil, errors.New("listRunningContainers should not be called when the daemon is down")
				},
			},
			want: "Docker:          not running\nDocker context:  desktop-linux\nMicrosandbox:    installed\nVMs running:     0\n",
		},
		{
			name:   "daemon running with no containers prints zero count",
			docker: &fakeDocker{context: "desktop-linux", listRunningContainers: noContainers},
			want:   "Docker:              running\nDocker context:      desktop-linux\nMicrosandbox:        installed\nContainers running:  0\nVMs running:         0\n",
		},
		{
			name: "counts running containers and VMs without naming them",
			docker: &fakeDocker{
				context: "prod",
				listRunningContainers: func() ([]*docker.ContainerInfo, error) {
					return []*docker.ContainerInfo{{Name: "agentic-claude-ab12", Tool: "claude"}}, nil
				},
			},
			msb: &fakeMicrosandbox{listRunning: func() ([]*microsandbox.Sandbox, error) {
				return []*microsandbox.Sandbox{{Name: "vm-1", Tool: "claude"}, {Name: "vm-2", Tool: "copilot"}}, nil
			}},
			want: "Docker:              running\nDocker context:      prod\nMicrosandbox:        installed\nContainers running:  1\nVMs running:         2\n",
		},
		{
			name:   "msb list error leaves out the VM count",
			docker: &fakeDocker{context: "prod", listRunningContainers: noContainers},
			msb: &fakeMicrosandbox{listRunning: func() ([]*microsandbox.Sandbox, error) {
				return nil, fmt.Errorf("msb error")
			}},
			want: "Docker:              running\nDocker context:      prod\nMicrosandbox:        installed\nContainers running:  0\n",
		},
		{
			name:   "msb not installed leaves out the VM count",
			docker: &fakeDocker{context: "prod", listRunningContainers: noContainers},
			msb:    &fakeMicrosandbox{available: func() error { return fmt.Errorf("msb not found") }},
			want:   "Docker:              running\nDocker context:      prod\nMicrosandbox:        not installed\nContainers running:  0\n",
		},
		{
			name: "context lookup error prints dash",
			docker: &fakeDocker{
				currentContext:        func() (string, error) { return "", fmt.Errorf("docker error") },
				listRunningContainers: noContainers,
			},
			want: "Docker:              running\nDocker context:      -\nMicrosandbox:        installed\nContainers running:  0\nVMs running:         0\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			stubDocker(t, tc.docker)
			stubMicrosandbox(t, cmp.Or(tc.msb, &fakeMicrosandbox{}))

			// Act
			var err error
			out := iostreamstest.CaptureOut(t, func() { err = runStatus(statusCmd, nil) })

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, out)
		})
	}

	t.Run("listRunningContainers error propagates", func(t *testing.T) {
		// Arrange
		stubMicrosandbox(t, &fakeMicrosandbox{})
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) {
			return nil, fmt.Errorf("docker error")
		})

		// Act
		var err error
		iostreamstest.CaptureOut(t, func() { err = runStatus(statusCmd, nil) })

		// Assert
		assert.ErrorContains(t, err, "docker error")
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
