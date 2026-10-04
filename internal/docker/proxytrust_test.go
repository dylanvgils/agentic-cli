package docker

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
)

func Test_proxyHandle_trustArgs(t *testing.T) {
	creds := []proxy.Credential{{Hosts: []string{"api.example.test"}}}

	t.Run("without credentials adds nothing", func(t *testing.T) {
		// Arrange
		handle := proxyHandle{caPEM: []byte("test-ca")}

		// Act
		args := handle.trustArgs()

		// Assert
		assert.Nil(t, args)
	})

	t.Run("passes the CA cert to the tool", func(t *testing.T) {
		// Arrange
		handle := proxyHandle{credentials: creds, caPEM: []byte("test-ca")}

		// Act
		args := handle.trustArgs()

		// Assert
		assert.Equal(t, []string{"--env=" + tools.ProxyCAEnvName + "=test-ca"}, args)
	})

	t.Run("before start shows a placeholder", func(t *testing.T) {
		// Arrange
		handle := proxyHandle{credentials: creds}

		// Act
		args := handle.trustArgs()

		// Assert
		assert.Equal(t, []string{"--env=" + tools.ProxyCAEnvName + "=" + dryRunCAPlaceholder}, args)
	})
}
