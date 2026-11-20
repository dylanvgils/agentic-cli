package sandbox

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatures_Missing(t *testing.T) {
	all := Features{
		Proxy:               true,
		ProxyMonitor:        true,
		Dind:                true,
		DindLimits:          true,
		CredentialInjection: true,
		PathRules:           true,
		ProxyIntegrations:   true,
		PidsLimit:           true,
		PortsWithProxy:      true,
	}

	typ := reflect.TypeFor[Features]()
	for i := range typ.NumField() {
		field := typ.Field(i)
		t.Run(field.Name, func(t *testing.T) {
			// Arrange
			require.Equal(t, reflect.Bool, field.Type.Kind(), "Missing only handles bool fields")
			var need, want Features
			reflect.ValueOf(&need).Elem().Field(i).SetBool(true)
			reflect.ValueOf(&want).Elem().Field(i).SetBool(true)

			// Act
			missing := Features{}.Missing(need)
			offered := all.Missing(need)

			// Assert
			assert.Equal(t, want, missing)
			assert.Equal(t, Features{}, offered)
		})
	}
}
