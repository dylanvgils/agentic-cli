package certs

import "crypto/x509"

// caPool returns a cert pool trusting only ca.
func caPool(ca KeyPair) *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)
	return pool
}
