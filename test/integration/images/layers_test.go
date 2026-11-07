//go:build integration

package images

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// TestLayerSmoke checks every extra base layer's runtime starts in the hardened container, through the version script
// the layer installs.
func TestLayerSmoke(t *testing.T) {
	t.Parallel()

	for _, name := range tools.KnownExtras() {
		t.Run(name+" prints its version", func(t *testing.T) {
			t.Parallel()

			// Act
			out := platform.NewToolRun(t).
				InNamespace(platform.LayersNamespace).
				WithDefaultMemory().
				Run("agentic-version-" + name)

			// Assert
			assert.Regexp(t, `\d+\.\d+\.\d+`, out)
		})
	}
}
