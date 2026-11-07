//go:build integration

package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestProxyStartup checks the image refuses a config it can't run safely.
func TestProxyStartup(t *testing.T) {
	env := newServiceEnv(t)

	t.Run("missing listen address exits", func(t *testing.T) {
		// Arrange
		run := newProxyRun(t, env).withoutAddr().create()

		// Act
		code, out := run.exitOutput()

		// Assert
		assert.Equal(t, "1", code)
		assert.Contains(t, out, "AGENTIC_PROXY_ADDR is not set")
	})

	t.Run("unscoped CA exits", func(t *testing.T) {
		// Arrange
		run := newProxyRun(t, env).
			allowing(credentialHost).
			withCredential().
			withUnscopedCA().
			create()

		// Act
		code, out := run.exitOutput()

		// Assert
		assert.Equal(t, "1", code)
		assert.Contains(t, out, "proxy CA must be name-constrained")
	})
}
