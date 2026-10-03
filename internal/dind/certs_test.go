package dind

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteCerts(t *testing.T) {
	dir := t.TempDir()
	err := WriteCerts(dir, []string{"agentic-docker", "127.0.0.1"})
	require.NoError(t, err)

	t.Run("writes only ca, cert and key per side", func(t *testing.T) {
		// Arrange
		ownDir := t.TempDir()

		// Act
		err := WriteCerts(ownDir, []string{"agentic-docker"})

		// Assert - in particular, no CA private key ever reaches disk
		require.NoError(t, err)
		var files []string
		walkErr := filepath.WalkDir(ownDir, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				rel, _ := filepath.Rel(ownDir, path)
				files = append(files, filepath.ToSlash(rel))
			}
			return err
		})
		require.NoError(t, walkErr)
		assert.ElementsMatch(t, []string{
			"client/ca.pem", "client/cert.pem", "client/key.pem",
			"server/ca.pem", "server/cert.pem", "server/key.pem",
		}, files)
	})

	t.Run("server cert verifies for the sidecar alias and loopback ip", func(t *testing.T) {
		// Arrange
		roots, leaf := loadCertSide(t, filepath.Join(dir, ServerCertSubdir))

		// Act
		_, aliasErr := leaf.Verify(x509.VerifyOptions{Roots: roots, DNSName: "agentic-docker", KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})
		_, ipErr := leaf.Verify(x509.VerifyOptions{Roots: roots, DNSName: "127.0.0.1", KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})

		// Assert
		assert.NoError(t, aliasErr)
		assert.NoError(t, ipErr)
	})

	t.Run("client cert is valid for client auth only", func(t *testing.T) {
		// Arrange
		roots, leaf := loadCertSide(t, filepath.Join(dir, ClientCertSubdir))

		// Act
		_, clientErr := leaf.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}})
		_, serverErr := leaf.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})

		// Assert
		assert.NoError(t, clientErr)
		assert.Error(t, serverErr)
	})
}
