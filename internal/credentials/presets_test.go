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
	}
}

func Test_bearer(t *testing.T) {
	// Act
	value := bearer("test-secret")

	// Assert
	assert.Equal(t, "Bearer test-secret", value)
}

func Test_gitBasic(t *testing.T) {
	// Act
	value := gitBasic("test-secret")

	// Assert
	assert.Equal(t, "Basic eC1hY2Nlc3MtdG9rZW46dGVzdC1zZWNyZXQ=", value)
}
