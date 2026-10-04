package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/certs"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_proxyHandle_prepareTrust(t *testing.T) {
	creds := []proxy.Credential{{Hosts: []string{"api.example.test"}}}

	t.Run("without credentials returns a zero CA and writes nothing", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		handle := proxyHandle{}

		// Act
		ca, err := handle.prepareTrust(RunSpec{ToolHome: toolHome})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, certs.CA{}, ca)
		assert.NoDirExists(t, filepath.Join(toolHome, proxyDirName))
	})

	t.Run("issues a scoped CA and writes the bundle and CA for the tool", func(t *testing.T) {
		// Arrange
		stubSystemCABundle(t, testSystemBundle, nil)
		handle := proxyHandle{credentials: creds, runDir: filepath.Join(t.TempDir(), "abc")}

		// Act
		ca, err := handle.prepareTrust(RunSpec{Image: "agentic-claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, ca.Scoped())
		bundle, err := os.ReadFile(filepath.Join(handle.runDir, proxyBundleFile))
		require.NoError(t, err)
		assert.Equal(t, testSystemBundle+string(ca.CertPEM()), string(bundle))
		toolCA, err := os.ReadFile(filepath.Join(handle.runDir, proxyCAFile))
		require.NoError(t, err)
		assert.Equal(t, ca.CertPEM(), toolCA)
	})

	t.Run("a bundle without a trailing newline still separates the CA", func(t *testing.T) {
		// Arrange
		stubSystemCABundle(t, "-----END CERTIFICATE-----", nil)
		handle := proxyHandle{credentials: creds, runDir: filepath.Join(t.TempDir(), "abc")}

		// Act
		ca, err := handle.prepareTrust(RunSpec{Image: "agentic-claude"})

		// Assert
		require.NoError(t, err)
		bundle, err := os.ReadFile(filepath.Join(handle.runDir, proxyBundleFile))
		require.NoError(t, err)
		assert.Equal(t, "-----END CERTIFICATE-----\n"+string(ca.CertPEM()), string(bundle))
	})

	t.Run("a bundle error leaves no run dir", func(t *testing.T) {
		// Arrange
		stubSystemCABundle(t, "", fmt.Errorf("stub: no bundle"))
		handle := proxyHandle{credentials: creds, runDir: filepath.Join(t.TempDir(), "abc")}

		// Act
		_, err := handle.prepareTrust(RunSpec{Image: "agentic-claude"})

		// Assert
		require.ErrorContains(t, err, "no bundle")
		assert.NoDirExists(t, handle.runDir)
	})
}

func Test_proxyHandle_trustArgs(t *testing.T) {
	t.Run("without credentials adds nothing", func(t *testing.T) {
		// Arrange
		handle := proxyHandle{runDir: "/agentic-home/proxy/abc"}

		// Act
		args := handle.trustArgs()

		// Assert
		assert.Nil(t, args)
	})

	t.Run("mounts the bundle and CA read-only and points Node at the CA", func(t *testing.T) {
		// Arrange
		handle := proxyHandle{credentials: []proxy.Credential{{Hosts: []string{"api.example.test"}}}, runDir: "/agentic-home/proxy/abc"}

		// Act
		args := handle.trustArgs()

		// Assert
		assert.Equal(t, []string{
			"--volume=" + filepath.Join(handle.runDir, proxyBundleFile) + ":/etc/ssl/certs/ca-certificates.crt:ro",
			"--volume=" + filepath.Join(handle.runDir, proxyCAFile) + ":/run/agentic/proxy-ca.pem:ro",
			"--env=NODE_EXTRA_CA_CERTS=/run/agentic/proxy-ca.pem",
		}, args)
	})
}
