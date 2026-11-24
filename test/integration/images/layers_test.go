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

// TestMavenProxySettings checks the java image's entrypoint renders the Maven proxy settings and points Maven at them.
func TestMavenProxySettings(t *testing.T) {
	t.Parallel()

	// Act
	out := platform.NewToolRun(t).
		InNamespace(platform.LayersNamespace).
		WithProxy().
		WithIntegrationFlag("maven").
		ViaEntrypoint().
		Run(`echo "maven=$MAVEN_ARGS"; grep -c '<port>3128</port>' "${MAVEN_ARGS#--global-settings }"`)

	// Assert
	assert.Contains(t, out, "maven=--global-settings /tmp/agentic-maven-settings.xml\n2\n")
}
