package dind

import (
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/certs"
)

// Subdirectories of a per-run cert dir.
const (
	ServerCertSubdir = "server"
	ClientCertSubdir = "client"
)

// WriteCerts generates a throwaway CA plus a server and client cert under dir; the CA key never touches disk.
func WriteCerts(dir string, serverNames []string) error {
	ca, err := certs.NewCA("agentic dind CA")
	if err != nil {
		return err
	}

	server, err := ca.IssueLeaf("agentic dind server", x509.ExtKeyUsageServerAuth, serverNames)
	if err != nil {
		return err
	}

	client, err := ca.IssueLeaf("agentic dind client", x509.ExtKeyUsageClientAuth, nil)
	if err != nil {
		return err
	}

	if err := writeCertDir(filepath.Join(dir, ServerCertSubdir), ca, server); err != nil {
		return err
	}
	return writeCertDir(filepath.Join(dir, ClientCertSubdir), ca, client)
}

// writeCertDir writes ca.pem, cert.pem and key.pem. Files are world-readable for the sidecar's
// user; the 0700 run dir keeps other host users out.
func writeCertDir(dir string, ca certs.CA, leaf certs.KeyPair) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create cert dir: %w", err)
	}

	keyPEM, err := leaf.KeyPEM()
	if err != nil {
		return err
	}

	files := map[string][]byte{
		"ca.pem":   ca.CertPEM(),
		"cert.pem": leaf.CertPEM(),
		"key.pem":  keyPEM,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	return nil
}
