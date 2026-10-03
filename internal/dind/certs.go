package dind

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

// certValidity bounds how long a per-run cert is usable; a leaked cert is worthless once the run's CA is gone anyway.
const certValidity = 30 * 24 * time.Hour

// Subdirectories of a per-run cert dir; the sidecar's entrypoint expects $DOCKER_TLS_CERTDIR/server.
const (
	ServerCertSubdir = "server"
	ClientCertSubdir = "client"
)

// keyPair is an issued certificate and its private key.
type keyPair struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

// certPEM returns the certificate PEM-encoded.
func (p keyPair) certPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: p.cert.Raw})
}

// keyPEM returns the private key PEM-encoded.
func (p keyPair) keyPEM() ([]byte, error) {
	der, err := x509.MarshalECPrivateKey(p.key)
	if err != nil {
		return nil, fmt.Errorf("marshal %s key: %w", p.cert.Subject.CommonName, err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), nil
}

// WriteCerts generates a throwaway CA plus a server and client cert under dir. The CA key
// only ever lives in memory, so nothing on disk can mint further certs for this daemon.
func WriteCerts(dir string, serverNames []string) error {
	ca, err := newCA()
	if err != nil {
		return err
	}

	server, err := newLeafCert(ca, "agentic dind server", x509.ExtKeyUsageServerAuth, serverNames)
	if err != nil {
		return err
	}

	client, err := newLeafCert(ca, "agentic dind client", x509.ExtKeyUsageClientAuth, nil)
	if err != nil {
		return err
	}

	if err := writeCertDir(filepath.Join(dir, ServerCertSubdir), ca, server); err != nil {
		return err
	}
	return writeCertDir(filepath.Join(dir, ClientCertSubdir), ca, client)
}

// newCA returns a self-signed CA that may only sign leaf certs.
func newCA() (keyPair, error) {
	tmpl, err := certTemplate("agentic dind CA")
	if err != nil {
		return keyPair{}, err
	}
	tmpl.IsCA = true
	tmpl.BasicConstraintsValid = true
	tmpl.MaxPathLenZero = true
	tmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature

	return issue(tmpl, nil)
}

// newLeafCert issues a cert signed by ca for one usage; names become DNS or IP SANs.
func newLeafCert(ca keyPair, cn string, usage x509.ExtKeyUsage, names []string) (keyPair, error) {
	tmpl, err := certTemplate(cn)
	if err != nil {
		return keyPair{}, err
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

	return issue(tmpl, &ca)
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
		NotAfter:     now.Add(certValidity),
	}, nil
}

// issue generates a key and signs tmpl with it, by parent or self-signed when parent is nil.
func issue(tmpl *x509.Certificate, parent *keyPair) (keyPair, error) {
	cn := tmpl.Subject.CommonName

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return keyPair{}, fmt.Errorf("generate %s key: %w", cn, err)
	}

	signer := keyPair{cert: tmpl, key: key}
	if parent != nil {
		signer = *parent
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, signer.cert, &key.PublicKey, signer.key)
	if err != nil {
		return keyPair{}, fmt.Errorf("create %s cert: %w", cn, err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return keyPair{}, fmt.Errorf("parse %s cert: %w", cn, err)
	}

	return keyPair{cert: cert, key: key}, nil
}

// writeCertDir writes ca.pem, cert.pem and key.pem in the layout the docker CLI and dind entrypoint expect.
// Files are world-readable because the sidecar's rootless user (uid 1000) may differ from the host
// user; the per-run parent dir is 0700, which keeps other host users out.
func writeCertDir(dir string, ca, leaf keyPair) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create cert dir: %w", err)
	}

	keyPEM, err := leaf.keyPEM()
	if err != nil {
		return err
	}

	files := map[string][]byte{
		"ca.pem":   ca.certPEM(),
		"cert.pem": leaf.certPEM(),
		"key.pem":  keyPEM,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	return nil
}
