//go:build windows

package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_resolveWindowsTimezone(t *testing.T) {
	for _, tc := range []struct {
		name    string
		windows string
		want    string
	}{
		{name: "known windows name maps to IANA name", windows: "Pacific Standard Time", want: "America/Los_Angeles"},
		{name: "unknown windows name returns empty", windows: "Not A Real Zone"},
		{name: "empty input returns empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			zone := resolveWindowsTimezone(tc.windows)

			// Assert
			assert.Equal(t, tc.want, zone)
		})
	}
}
