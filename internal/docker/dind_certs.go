package docker

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// dindCertValidity bounds how long a per-run cert is usable; a leaked cert is worthless once the run's CA is gone anyway.
const dindCertValidity = 30 * 24 * time.Hour

// Subdirectories of a per-run cert dir; the sidecar's entrypoint expects $DOCKER_TLS_CERTDIR/server.
const (
	dindServerCertSubdir = "server"
	dindClientCertSubdir = "client"
)

// certPair is a signed certificate and its private key, PEM-encoded.
type certPair struct {
	certPEM []byte
	keyPEM  []byte
}

// writeDindCerts generates a throwaway CA plus a server and client cert under dir. The CA key
// only ever lives in memory, so nothing on disk can mint further certs for this daemon.
func writeDindCerts(dir string, serverNames []string) error {
	ca, caKey, caPEM, err := newCA()
	if err != nil {
		return err
	}

	server, err := newLeafCert(ca, caKey, "agentic dind server", x509.ExtKeyUsageServerAuth, serverNames)
	if err != nil {
		return err
	}

	client, err := newLeafCert(ca, caKey, "agentic dind client", x509.ExtKeyUsageClientAuth, nil)
	if err != nil {
		return err
	}

	if err := writeCertDir(filepath.Join(dir, dindServerCertSubdir), caPEM, server); err != nil {
		return err
	}
	return writeCertDir(filepath.Join(dir, dindClientCertSubdir), caPEM, client)
}

// newCA returns a self-signed CA cert, its in-memory key, and the cert as PEM.
func newCA() (*x509.Certificate, *ecdsa.PrivateKey, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generate dind CA key: %w", err)
	}

	tmpl, err := certTemplate("agentic dind CA")
	if err != nil {
		return nil, nil, nil, err
	}
	tmpl.IsCA = true
	tmpl.BasicConstraintsValid = true
	tmpl.MaxPathLenZero = true
	tmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create dind CA cert: %w", err)
	}

	ca, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse dind CA cert: %w", err)
	}

	return ca, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}

// newLeafCert issues a cert signed by ca for one usage; names become DNS or IP SANs.
func newLeafCert(ca *x509.Certificate, caKey *ecdsa.PrivateKey, cn string, usage x509.ExtKeyUsage, names []string) (certPair, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return certPair{}, fmt.Errorf("generate %s key: %w", cn, err)
	}

	tmpl, err := certTemplate(cn)
	if err != nil {
		return certPair{}, err
	}
	tmpl.KeyUsage = x509.KeyUsageDigitalSignature
	tmpl.ExtKeyUsage = []x509.ExtKeyUsage{usage}

	for _, name := range names {
		if ip := net.ParseIP(name); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, name)
		}
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		return certPair{}, fmt.Errorf("create %s cert: %w", cn, err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return certPair{}, fmt.Errorf("marshal %s key: %w", cn, err)
	}

	return certPair{
		certPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		keyPEM:  pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
	}, nil
}

// certTemplate returns a template with a random serial and the per-run validity window.
func certTemplate(cn string) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate cert serial: %w", err)
	}

	now := time.Now()
	return &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(dindCertValidity),
	}, nil
}

// writeCertDir writes ca.pem, cert.pem and key.pem in the layout the docker CLI and dind entrypoint expect.
// Files are world-readable because the sidecar's rootless user (uid 1000) may differ from the host
// user; the per-run parent dir is 0700, which keeps other host users out.
func writeCertDir(dir string, caPEM []byte, pair certPair) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create cert dir: %w", err)
	}

	files := map[string][]byte{
		"ca.pem":   caPEM,
		"cert.pem": pair.certPEM,
		"key.pem":  pair.keyPEM,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	return nil
}
