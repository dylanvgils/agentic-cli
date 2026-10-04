// Package certs issues throwaway per-run CAs and leaf certs. It imports only the standard library so the agentic-proxy binary may use it.
package certs

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
	"time"
)

// certValidity bounds how long a per-run cert is usable.
const certValidity = 30 * 24 * time.Hour

// KeyPair is an issued certificate and its private key.
type KeyPair struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

// IssueLeaf issues a cert signed by p (a CA from NewCA) for one usage; names become DNS or IP SANs.
func (p KeyPair) IssueLeaf(cn string, usage x509.ExtKeyUsage, names []string) (KeyPair, error) {
	tmpl, err := certTemplate(cn)
	if err != nil {
		return KeyPair{}, err
	}
	tmpl.KeyUsage = x509.KeyUsageDigitalSignature
	tmpl.ExtKeyUsage = []x509.ExtKeyUsage{usage}
	addSANs(tmpl, names)

	return issue(tmpl, &p)
}

// CertPEM returns the certificate PEM-encoded.
func (p KeyPair) CertPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: p.cert.Raw})
}

// KeyPEM returns the private key PEM-encoded.
func (p KeyPair) KeyPEM() ([]byte, error) {
	der, err := x509.MarshalECPrivateKey(p.key)
	if err != nil {
		return nil, fmt.Errorf("marshal %s key: %w", p.cert.Subject.CommonName, err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), nil
}

// NewCA returns a self-signed CA named cn that may only sign leaf certs.
func NewCA(cn string) (KeyPair, error) {
	tmpl, err := certTemplate(cn)
	if err != nil {
		return KeyPair{}, err
	}
	tmpl.IsCA = true
	tmpl.BasicConstraintsValid = true
	tmpl.MaxPathLenZero = true
	tmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature

	return issue(tmpl, nil)
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

// issue generates a key and signs tmpl by parent, or self-signs when parent is nil.
func issue(tmpl *x509.Certificate, parent *KeyPair) (KeyPair, error) {
	cn := tmpl.Subject.CommonName

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return KeyPair{}, fmt.Errorf("generate %s key: %w", cn, err)
	}

	signer := KeyPair{cert: tmpl, key: key}
	if parent != nil {
		signer = *parent
	}

	cert, err := sign(tmpl, signer, &key.PublicKey)
	if err != nil {
		return KeyPair{}, err
	}

	return KeyPair{cert: cert, key: key}, nil
}

// sign creates the certificate for pub from tmpl, signed by signer.
func sign(tmpl *x509.Certificate, signer KeyPair, pub *ecdsa.PublicKey) (*x509.Certificate, error) {
	cn := tmpl.Subject.CommonName

	der, err := x509.CreateCertificate(rand.Reader, tmpl, signer.cert, pub, signer.key)
	if err != nil {
		return nil, fmt.Errorf("create %s cert: %w", cn, err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse %s cert: %w", cn, err)
	}

	return cert, nil
}

// addSANs adds each name to tmpl as an IP SAN if it parses as one, else as a DNS SAN.
func addSANs(tmpl *x509.Certificate, names []string) {
	for _, name := range names {
		if ip := net.ParseIP(name); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, name)
		}
	}
}
