//go:build windows

package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveWindowsTimezone(t *testing.T) {
	t.Run("known windows name maps to IANA name", func(t *testing.T) {
		// Act
		zone := resolveWindowsTimezone("Pacific Standard Time")

		// Assert
		assert.Equal(t, "America/Los_Angeles", zone)
	})

	t.Run("unknown windows name returns empty", func(t *testing.T) {
		// Act
		zone := resolveWindowsTimezone("Not A Real Zone")

		// Assert
		assert.Empty(t, zone)
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		// Act
		zone := resolveWindowsTimezone("")

		// Assert
		assert.Empty(t, zone)
	})
}
