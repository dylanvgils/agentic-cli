//go:build !windows

package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLocaltimeSymlink(t *testing.T) {
	t.Run("extracts zone name after zoneinfo segment", func(t *testing.T) {
		// Arrange
		target := "/usr/share/zoneinfo/America/New_York"

		// Act
		zone := parseLocaltimeSymlink(target)

		// Assert
		assert.Equal(t, "America/New_York", zone)
	})

	t.Run("no zoneinfo segment returns empty", func(t *testing.T) {
		// Arrange
		target := "/some/other/path"

		// Act
		zone := parseLocaltimeSymlink(target)

		// Assert
		assert.Empty(t, zone)
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		// Act
		zone := parseLocaltimeSymlink("")

		// Assert
		assert.Empty(t, zone)
	})
}

func TestParseLocaltimePosixFooter(t *testing.T) {
	t.Run("extracts POSIX TZ string from TZif footer", func(t *testing.T) {
		// Arrange
		data := []byte("TZif2\x00...binary header and body...\nCET-1CEST,M3.5.0,M10.5.0/3\n")

		// Act
		tz := parseLocaltimePosixFooter(data)

		// Assert
		assert.Equal(t, "CET-1CEST,M3.5.0,M10.5.0/3", tz)
	})

	t.Run("no trailing newline returns empty", func(t *testing.T) {
		// Arrange
		data := []byte("TZif2\x00...binary header and body...")

		// Act
		tz := parseLocaltimePosixFooter(data)

		// Assert
		assert.Empty(t, tz)
	})

	t.Run("single newline returns empty", func(t *testing.T) {
		// Arrange
		data := []byte("no footer delimiter here\n")

		// Act
		tz := parseLocaltimePosixFooter(data)

		// Assert
		assert.Empty(t, tz)
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		// Act
		tz := parseLocaltimePosixFooter(nil)

		// Assert
		assert.Empty(t, tz)
	})
}
