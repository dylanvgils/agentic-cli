package certs

import (
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/require"
)

// newTestCA returns a fresh CA, failing the test on error.
func newTestCA(t *testing.T) CA {
	t.Helper()
	ca, err := NewCA("test CA")
	require.NoError(t, err)
	return ca
}

// issueTestLeaf returns a server leaf signed by ca, failing the test on error.
func issueTestLeaf(t *testing.T, ca CA, names ...string) KeyPair {
	t.Helper()
	leaf, err := ca.IssueLeaf("test leaf", x509.ExtKeyUsageServerAuth, names)
	require.NoError(t, err)
	return leaf
}

// verifyLeaf checks leaf against a pool trusting only ca, for server auth and (if set) dnsName.
func verifyLeaf(ca CA, leaf KeyPair, dnsName string) error {
	roots := x509.NewCertPool()
	roots.AddCert(ca.cert)

	_, err := leaf.cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: dnsName, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})
	return err
}
