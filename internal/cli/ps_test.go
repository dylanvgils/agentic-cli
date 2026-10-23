package cli

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runPs(t *testing.T) {
	// Keep the repo's own .agenticrc.toml out of the test
	t.Chdir(t.TempDir())
	stubCheckDockerDaemon(t, func() error { return nil })
	stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) {
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
	})
	stubMicrosandbox(t, &fakeMicrosandbox{listRunning: func() ([]*microsandbox.Sandbox, error) {
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

	t.Run("lists the active namespace's runs on every runtime", func(t *testing.T) {
		// Arrange
		buf := stubLogs(t)
		cmd := newTestPsCmd(t)

		// Act
		err := runPs(cmd, nil)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Regexp(t, `claude\s+agentic\s+docker\s+agentic-claude\s+Up 5 minutes\s+agentic-claude-ab12`, out)
		assert.Regexp(t, `opencode\s+agentic\s+microsandbox\s+agentic-opencode:latest\s+running\s+vm-1`, out)
		assert.NotContains(t, out, "other-copilot-cd34")
	})

	t.Run("lists every namespace with --all", func(t *testing.T) {
		// Arrange
		buf := stubLogs(t)
		cmd := newTestPsCmd(t, "--all")

		// Act
		err := runPs(cmd, nil)

		// Assert
		require.NoError(t, err)
		out := buf.String()
		assert.Regexp(t, `claude\s+agentic\s+docker\s+agentic-claude\s+Up 5 minutes\s+agentic-claude-ab12`, out)
		assert.Regexp(t, `copilot\s+other\s+docker\s+other-copilot\s+Up 1 minute\s+other-copilot-cd34`, out)
	})

	t.Run("lists the namespace given with --namespace", func(t *testing.T) {
		// Arrange
		buf := stubLogs(t)
		cmd := newTestPsCmd(t, "--namespace", "other")

		// Act
		err := runPs(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "other-copilot-cd34")
		assert.NotContains(t, buf.String(), "agentic-claude-ab12")
	})

	t.Run("names the namespace when nothing runs in it", func(t *testing.T) {
		// Arrange
		buf := stubLogs(t)
		cmd := newTestPsCmd(t, "--namespace", "empty")

		// Act
		err := runPs(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "no running containers or VMs in namespace \"empty\"")
	})

	t.Run("says so when nothing runs in any namespace", func(t *testing.T) {
		// Arrange
		buf := stubLogs(t)
		cmd := newTestPsCmd(t, "--all")
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) { return nil, nil })
		stubMicrosandbox(t, &fakeMicrosandbox{})

		// Act
		err := runPs(cmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "no running containers or VMs\n")
	})
}
