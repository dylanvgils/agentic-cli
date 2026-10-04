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

func TestNewScopedCA(t *testing.T) {
	t.Run("leaf for a permitted name or its subdomain verifies", func(t *testing.T) {
		// Arrange
		ca, err := NewScopedCA("test CA", []string{"api.example.test"})
		require.NoError(t, err)

		for _, name := range []string{"api.example.test", "eu.api.example.test"} {
			leaf := issueTestLeaf(t, ca, x509.ExtKeyUsageServerAuth, name)

			// Act
			err := verifyLeaf(ca, leaf, x509.ExtKeyUsageServerAuth, name)

			// Assert
			assert.NoError(t, err, name)
		}
	})

	t.Run("leaf for any other name is rejected", func(t *testing.T) {
		// Arrange
		ca, err := NewScopedCA("test CA", []string{"api.example.test"})
		require.NoError(t, err)

		for _, name := range []string{"example.test", "evil.test", "127.0.0.1", "::1"} {
			leaf := issueTestLeaf(t, ca, x509.ExtKeyUsageServerAuth, name)

			// Act
			err := verifyLeaf(ca, leaf, x509.ExtKeyUsageServerAuth, name)

			// Assert
			assert.ErrorContains(t, err, "not authorized to sign for this name", name)
		}
	})

	t.Run("ip-only scope permits only those ips", func(t *testing.T) {
		// Arrange
		ca, err := NewScopedCA("test CA", []string{"127.0.0.1", "::1"})
		require.NoError(t, err)
		cases := map[string]bool{"127.0.0.1": true, "::1": true, "127.0.0.2": false, "::2": false, "api.example.test": false}

		for name, permitted := range cases {
			leaf := issueTestLeaf(t, ca, x509.ExtKeyUsageServerAuth, name)

			// Act
			err := verifyLeaf(ca, leaf, x509.ExtKeyUsageServerAuth, name)

			// Assert
			if permitted {
				assert.NoError(t, err, name)
			} else {
				assert.ErrorContains(t, err, "not authorized to sign for this name", name)
			}
		}
	})

	t.Run("constraints are critical and survive a reload", func(t *testing.T) {
		// Arrange
		ca, err := NewScopedCA("test CA", []string{"api.example.test"})
		require.NoError(t, err)
		keyPEM, err := ca.KeyPEM()
		require.NoError(t, err)

		// Act
		loaded, err := LoadCA(ca.CertPEM(), keyPEM)

		// Assert
		require.NoError(t, err)
		assert.True(t, loaded.cert.PermittedDNSDomainsCritical)
		assert.Equal(t, []string{"api.example.test"}, loaded.cert.PermittedDNSDomains)
		assert.True(t, loaded.cert.IsCA)
		assert.True(t, loaded.cert.MaxPathLenZero)
	})

	t.Run("no names is an error", func(t *testing.T) {
		// Act
		_, err := NewScopedCA("test CA", nil)

		// Assert
		assert.ErrorContains(t, err, "needs at least one name")
	})
}

func TestCAScoped(t *testing.T) {
	t.Run("scoped ca reports scoped", func(t *testing.T) {
		for _, name := range []string{"127.0.0.1", "api.example.test"} {
			// Arrange
			ca, err := NewScopedCA("test CA", []string{name})
			require.NoError(t, err)

			// Act
			scoped := ca.Scoped()

			// Assert
			assert.True(t, scoped, name)
		}
	})

	t.Run("dns constraints without ip constraints is not scoped", func(t *testing.T) {
		// Arrange
		tmpl, err := caTemplate("test CA")
		require.NoError(t, err)
		tmpl.PermittedDNSDomainsCritical = true
		tmpl.PermittedDNSDomains = []string{"api.example.test"}
		ca, err := selfSign(tmpl)
		require.NoError(t, err)

		// Act
		scoped := ca.Scoped()

		// Assert
		assert.False(t, scoped)
	})

	t.Run("plain ca is not scoped", func(t *testing.T) {
		// Arrange
		ca := newTestCA(t)

		// Act
		scoped := ca.Scoped()

		// Assert
		assert.False(t, scoped)
	})
}

func TestIssueLeaf(t *testing.T) {
	ca := newTestCA(t)

	t.Run("leaf verifies against the ca for its usage", func(t *testing.T) {
		// Act
		leaf, err := ca.IssueLeaf("server", x509.ExtKeyUsageServerAuth, []string{"example.test"})

		// Assert
		require.NoError(t, err)
		assert.NoError(t, verifyLeaf(ca, leaf, x509.ExtKeyUsageServerAuth, "example.test"))
		assert.False(t, leaf.cert.IsCA)
	})

	t.Run("leaf is rejected for another usage", func(t *testing.T) {
		// Act
		leaf, err := ca.IssueLeaf("client", x509.ExtKeyUsageClientAuth, nil)

		// Assert
		require.NoError(t, err)
		assert.Error(t, verifyLeaf(ca, leaf, x509.ExtKeyUsageServerAuth, ""))
	})
}

func TestKeyPairPEM(t *testing.T) {
	// Arrange
	leaf := issueTestLeaf(t, newTestCA(t), x509.ExtKeyUsageServerAuth, "example.test")

	// Act
	keyPEM, err := leaf.KeyPEM()

	// Assert
	require.NoError(t, err)
	pair, pairErr := tls.X509KeyPair(leaf.CertPEM(), keyPEM)
	require.NoError(t, pairErr)
	assert.Equal(t, leaf.cert.Raw, pair.Certificate[0])
}

func TestKeyPairTLSCertificate(t *testing.T) {
	// Arrange
	leaf := issueTestLeaf(t, newTestCA(t), x509.ExtKeyUsageServerAuth, "example.test")

	// Act
	cert := leaf.TLSCertificate()

	// Assert
	assert.Equal(t, [][]byte{leaf.cert.Raw}, cert.Certificate)
	assert.Equal(t, leaf.key, cert.PrivateKey)
	assert.Same(t, leaf.cert, cert.Leaf)
}

func TestLoadCA(t *testing.T) {
	ca := newTestCA(t)
	caKeyPEM, err := ca.KeyPEM()
	require.NoError(t, err)

	t.Run("round-trips a ca that can issue leaves", func(t *testing.T) {
		// Act
		loaded, err := LoadCA(ca.CertPEM(), caKeyPEM)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, ca.cert.Raw, loaded.cert.Raw)
		leaf := issueTestLeaf(t, loaded, x509.ExtKeyUsageServerAuth, "example.test")
		assert.NoError(t, verifyLeaf(ca, leaf, x509.ExtKeyUsageServerAuth, "example.test"))
	})

	t.Run("rejects a non-ca cert", func(t *testing.T) {
		// Arrange
		leaf := issueTestLeaf(t, ca, x509.ExtKeyUsageServerAuth, "example.test")
		leafKeyPEM, err := leaf.KeyPEM()
		require.NoError(t, err)

		// Act
		_, err = LoadCA(leaf.CertPEM(), leafKeyPEM)

		// Assert
		assert.ErrorContains(t, err, "not a CA")
	})

	t.Run("rejects a key from another ca", func(t *testing.T) {
		// Arrange
		otherKeyPEM, err := newTestCA(t).KeyPEM()
		require.NoError(t, err)

		// Act
		_, err = LoadCA(ca.CertPEM(), otherKeyPEM)

		// Assert
		assert.ErrorContains(t, err, "does not match")
	})

	t.Run("rejects non-pem input", func(t *testing.T) {
		// Act
		_, err := LoadCA([]byte("garbage"), caKeyPEM)

		// Assert
		assert.ErrorContains(t, err, "no CERTIFICATE PEM block")
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

func Test_decodePEM(t *testing.T) {
	ca := newTestCA(t)

	t.Run("returns the bytes of a matching block", func(t *testing.T) {
		// Act
		der, err := decodePEM(ca.CertPEM(), "CERTIFICATE")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, ca.cert.Raw, der)
	})

	t.Run("rejects a block of another type", func(t *testing.T) {
		// Act
		_, err := decodePEM(ca.CertPEM(), "EC PRIVATE KEY")

		// Assert
		assert.ErrorContains(t, err, "no EC PRIVATE KEY PEM block")
	})
}
