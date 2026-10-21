package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCapabilities_Missing(t *testing.T) {
	all := Capabilities{Proxy: true, ProxyMonitor: true, Dind: true, CredentialInjection: true, PathRules: true, SecretMounts: true, PidsLimit: true}

	t.Run("lists what the need asks for and c lacks", func(t *testing.T) {
		// Arrange
		offered := Capabilities{CredentialInjection: true}

		// Act
		missing := offered.Missing(all)

		// Assert
		assert.Equal(t, Capabilities{Proxy: true, ProxyMonitor: true, Dind: true, PathRules: true, SecretMounts: true, PidsLimit: true}, missing)
	})

	t.Run("nothing is missing for an empty need", func(t *testing.T) {
		// Act
		missing := Capabilities{}.Missing(Capabilities{})

		// Assert
		assert.Equal(t, Capabilities{}, missing)
	})

	t.Run("credentials via the proxy are never missing", func(t *testing.T) {
		// Act
		missing := Capabilities{}.Missing(Capabilities{CredentialsViaProxy: true})

		// Assert
		assert.Equal(t, Capabilities{}, missing)
	})
}
