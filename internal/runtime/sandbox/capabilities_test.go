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
	}

	t.Run("lists what the need asks for and f lacks", func(t *testing.T) {
		// Arrange
		offered := Features{CredentialInjection: true}

		// Act
		missing := offered.Missing(all)

		// Assert
		assert.Equal(t, Features{
			Proxy:             true,
			ProxyMonitor:      true,
			Dind:              true,
			DindLimits:        true,
			PathRules:         true,
			ProxyIntegrations: true,
			PidsLimit:         true,
		}, missing)
	})

	t.Run("nothing is missing for an empty need", func(t *testing.T) {
		// Act
		missing := Features{}.Missing(Features{})

		// Assert
		assert.Equal(t, Features{}, missing)
	})

	t.Run("every field is compared", func(t *testing.T) {
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
	})
}
