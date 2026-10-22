package credentials

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_presets(t *testing.T) {
	// Arrange
	secret := writeSecret(t, "test-secret")
	var entries []config.RCCredential
	for name := range presets {
		entries = append(entries, config.RCCredential{Preset: name, Secret: secret})
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

func Test_presets_github(t *testing.T) {
	// Act
	github := presets["github"]

	// Assert
	assert.Contains(t, github.hosts, "github.com")
	assert.Contains(t, github.hosts, "api.githubcopilot.com")
	assert.Contains(t, github.hosts, "api.individual.githubcopilot.com")
}
