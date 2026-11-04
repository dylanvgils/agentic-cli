package docker

import (
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/require"
)

func TestClient_dryRun(t *testing.T) {
	client := newTestClient()
	stubDockerRunCapture(t, client)
	stubDryRunEnv(t)

	t.Run("prints every sidecar and the tool", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		rs := sandbox.RunSpec{
			Image:         "agentic-claude",
			ToolHome:      toolHome,
			ContainerHome: "/home/claude",
			User:          &testUser,
			Volumes:       []string{"$PWD:/workspace"},
			Env:           []string{"EXAMPLE=value"},
			ToolArgs:      []string{"--example"},
			Limits:        sandbox.ResourceLimits{PidsLimit: "512", CPUs: "2", Memory: "2g"},
			DryRun:        true,
			Credentials: []sandbox.Credential{{
				Name:    "EXAMPLE_API_KEY",
				Aliases: []string{"EXAMPLE_TOKEN"},
				Hosts:   []string{"api.example.test"},
				Headers: []string{"X-Api-Key"},
				Secret:  "test-secret",
			}},
			Proxy: sandbox.ProxySpec{
				Mode: sandbox.ProxyEnforce,
				Rules: []sandbox.AllowRule{
					{Host: "api.example.test"},
					{Host: "repo.example.test", Paths: []string{"/org/**"}},
				},
				LogFile:      filepath.Join(toolHome, "logs", "proxy_0123456789ab.jsonl"),
				Integrations: []string{"gradle", "maven"},
			},
			Dind: sandbox.DindSpec{
				Enabled: true,
				Limits:  sandbox.ResourceLimits{PidsLimit: "256", CPUs: "1", Memory: "1g"},
			},
		}

		// Act
		var err error
		out := iostreamstest.CaptureOut(t, func() { err = startAndRunTool(client, rs) })

		// Assert
		require.NoError(t, err)
		assertGolden(t, "dryrun_enforce.golden", normalizeDryRun(t, out, toolHome))
	})

	t.Run("prints a monitor-mode proxy", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		rs := sandbox.RunSpec{
			Image:         "agentic-claude",
			ToolHome:      toolHome,
			ContainerHome: "/home/claude",
			User:          &testUser,
			DryRun:        true,
			Proxy: sandbox.ProxySpec{
				Mode:    sandbox.ProxyMonitor,
				Rules:   []sandbox.AllowRule{{Host: "api.example.test"}},
				LogFile: filepath.Join(toolHome, "logs", "proxy_0123456789ab.jsonl"),
			},
		}

		// Act
		var err error
		out := iostreamstest.CaptureOut(t, func() { err = startAndRunTool(client, rs) })

		// Assert
		require.NoError(t, err)
		assertGolden(t, "dryrun_monitor.golden", normalizeDryRun(t, out, toolHome))
	})
}
