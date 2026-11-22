package credentials

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config/rc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_presets(t *testing.T) {
	// Arrange
	secret := writeSecret(t, "test-secret")
	var entries []rc.RCCredential
	for name := range presets {
		entries = append(entries, rc.RCCredential{Preset: name, Secret: secret})
	}

	// Act
	resolved, err := Resolve(entries)

	// Assert
	require.NoError(t, err, "every preset passes the proxy's validation and claims distinct hosts")
	for _, r := range resolved {
		assert.NotEmpty(t, r.Env)
		assert.NotEmpty(t, r.Headers)
	}
}
