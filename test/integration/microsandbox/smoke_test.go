//go:build integration

package microsandbox

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// TestMicrosandboxToolSmoke checks every tool's image starts the tool through its entrypoint in a microsandbox VM, with
// the default memory limit, since copilot is OOM-killed under the work dir's.
func TestMicrosandboxToolSmoke(t *testing.T) {
	for _, name := range tools.Names() {
		t.Run(name+" prints its version", func(t *testing.T) {
			// Act
			out := platform.NewToolRun(t).
				ForTool(name).
				InMicrosandbox().
				WithDefaultMemory().
				RunTool("--version")

			// Assert
			assert.Regexp(t, `\d+\.\d+\.\d+`, out)
		})
	}
}

// TestMicrosandboxLayerSmoke checks every extra base layer's runtime starts in a microsandbox VM, through the version
// script the layer installs.
func TestMicrosandboxLayerSmoke(t *testing.T) {
	for _, name := range tools.KnownExtras() {
		t.Run(name+" prints its version", func(t *testing.T) {
			// Act
			out := platform.NewToolRun(t).
				InNamespace(platform.LayersNamespace).
				InMicrosandbox().
				WithDefaultMemory().
				Run("agentic-version-" + name)

			// Assert
			assert.Regexp(t, `\d+\.\d+\.\d+`, out)
		})
	}
}
