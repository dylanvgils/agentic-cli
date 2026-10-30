//go:build integration

package tools

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// TestToolSmoke checks every tool's image starts the tool through its entrypoint in the hardened container, with the
// default memory limit, since copilot is OOM-killed under the work dir's.
func TestToolSmoke(t *testing.T) {
	for _, name := range tools.Names() {
		t.Run(name+" prints its version", func(t *testing.T) {
			// Act
			out := platform.NewToolRun(t).
				ForTool(name).
				WithDefaultMemory().
				RunTool("--version")

			// Assert
			assert.Regexp(t, `\d+\.\d+\.\d+`, out)
		})
	}
}
