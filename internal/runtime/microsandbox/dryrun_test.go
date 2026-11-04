package microsandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/require"
)

func TestClient_Run_dryRun(t *testing.T) {
	client := newTestClient(&fakeRunner{})
	stubDryRunEnv(t)

	t.Run("prints the VM with egress, credentials and a Docker daemon", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(toolHome, "project"), 0o750))
		rs := sandbox.RunSpec{
			Image:          "agentic-claude-dockerd",
			ToolHome:       toolHome,
			ContainerHome:  "/home/claude",
			Volumes:        []string{"$TOOL_HOME/project:/workspace", "$TOOL_HOME/project:/mnt/ro:ro"},
			Env:            []string{"EXAMPLE=value"},
			SkipEntrypoint: true,
			ToolArgs:       []string{"bash", "-c", "true"},
			Limits:         sandbox.ResourceLimits{CPUs: "1.5", Memory: "2g"},
			DryRun:         true,
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
					{Host: ".example.test"},
				},
			},
			Dind: sandbox.DindSpec{Enabled: true},
		}

		// Act
		var err error
		out := iostreamstest.CaptureOut(t, func() { err = client.Run(rs) })

		// Assert
		require.NoError(t, err)
		assertGolden(t, "dryrun.golden", normalizeDryRun(out, toolHome))
	})
}
