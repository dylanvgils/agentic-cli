package certs

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCA(t *testing.T) {
	// Act
	ca, err := NewCA("test CA")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "test CA", ca.cert.Subject.CommonName)
	assert.True(t, ca.cert.IsCA)
	assert.True(t, ca.cert.MaxPathLenZero)
	assert.NotZero(t, ca.cert.KeyUsage&x509.KeyUsageCertSign)
	assert.NoError(t, ca.cert.CheckSignatureFrom(ca.cert))
}

func TestIssueLeaf(t *testing.T) {
	ca, err := NewCA("test CA")
	require.NoError(t, err)

	t.Run("leaf verifies against the ca for its usage", func(t *testing.T) {
		// Act
		leaf, err := ca.IssueLeaf("server", x509.ExtKeyUsageServerAuth, []string{"example.test"})

		// Assert
		require.NoError(t, err)
		_, verifyErr := leaf.cert.Verify(x509.VerifyOptions{Roots: caPool(ca), DNSName: "example.test", KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})
		assert.NoError(t, verifyErr)
		assert.False(t, leaf.cert.IsCA)
	})

	t.Run("leaf is rejected for another usage", func(t *testing.T) {
		// Act
		leaf, err := ca.IssueLeaf("client", x509.ExtKeyUsageClientAuth, nil)

		// Assert
		require.NoError(t, err)
		_, verifyErr := leaf.cert.Verify(x509.VerifyOptions{Roots: caPool(ca), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})
		assert.Error(t, verifyErr)
	})
}

func Test_addSANs(t *testing.T) {
	// Arrange
	tmpl := &x509.Certificate{}

	// Act
	addSANs(tmpl, []string{"example.test", "127.0.0.1", "::1"})

	// Assert
	assert.Equal(t, []string{"example.test"}, tmpl.DNSNames)
	require.Len(t, tmpl.IPAddresses, 2)
	assert.True(t, tmpl.IPAddresses[0].Equal(net.ParseIP("127.0.0.1")))
	assert.True(t, tmpl.IPAddresses[1].Equal(net.ParseIP("::1")))
}

func TestKeyPairPEM(t *testing.T) {
	// Arrange
	ca, err := NewCA("test CA")
	require.NoError(t, err)
	leaf, err := ca.IssueLeaf("server", x509.ExtKeyUsageServerAuth, []string{"example.test"})
	require.NoError(t, err)

	// Act
	keyPEM, err := leaf.KeyPEM()

	// Assert
	require.NoError(t, err)
	pair, pairErr := tls.X509KeyPair(leaf.CertPEM(), keyPEM)
	require.NoError(t, pairErr)
	assert.Equal(t, leaf.cert.Raw, pair.Certificate[0])
}
