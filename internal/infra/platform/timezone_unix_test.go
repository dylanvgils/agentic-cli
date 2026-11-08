//go:build !windows

package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseLocaltimeSymlink(t *testing.T) {
	for _, tc := range []struct {
		name   string
		target string
		want   string
	}{
		{name: "extracts zone name after zoneinfo segment", target: "/usr/share/zoneinfo/America/New_York", want: "America/New_York"},
		{name: "no zoneinfo segment returns empty", target: "/some/other/path"},
		{name: "empty input returns empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			zone := parseLocaltimeSymlink(tc.target)

			// Assert
			assert.Equal(t, tc.want, zone)
		})
	}
}

func Test_parseLocaltimePosixFooter(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
		want string
	}{
		{name: "extracts POSIX TZ string from TZif footer", data: "TZif2\x00...binary header and body...\nCET-1CEST,M3.5.0,M10.5.0/3\n", want: "CET-1CEST,M3.5.0,M10.5.0/3"},
		{name: "no trailing newline returns empty", data: "TZif2\x00...binary header and body..."},
		{name: "single newline returns empty", data: "no footer delimiter here\n"},
		{name: "empty input returns empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			tz := parseLocaltimePosixFooter([]byte(tc.data))

			// Assert
			assert.Equal(t, tc.want, tz)
		})
	}
}
