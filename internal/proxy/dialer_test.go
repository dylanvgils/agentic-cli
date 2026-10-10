package proxy

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_blockedAddr(t *testing.T) {
	local := []string{"127.0.0.1", "::1", "169.254.169.254", "fe80::1", "0.0.0.0", "0.1.2.3", "::", "224.0.0.1", "ff02::1", "::ffff:127.0.0.1",
		"fd00:ec2::254", "100.100.100.200", "168.63.129.16", "::7f00:1", "fec0::1", "64:ff9b::7f00:1", "64:ff9b::a9fe:a9fe", "2002:7f00:1::"}
	private := []string{"10.0.0.1", "172.17.0.1", "192.168.1.1", "100.64.0.1", "fc00::1", "::ffff:10.0.0.1",
		"192.0.0.1", "198.18.0.1", "64:ff9b::a00:1", "2002:a00:1::"}
	public := []string{"203.0.113.10", "2001:db8::1", "64:ff9b::cb00:710a"}

	for _, raw := range local {
		t.Run("refuses "+raw+" even with privateOK", func(t *testing.T) {
			// Act
			blocked := blockedAddr(netip.MustParseAddr(raw), true)

			// Assert
			assert.True(t, blocked)
		})
	}

	for _, raw := range private {
		t.Run("refuses "+raw+" only without privateOK", func(t *testing.T) {
			// Act
			withoutOK := blockedAddr(netip.MustParseAddr(raw), false)
			withOK := blockedAddr(netip.MustParseAddr(raw), true)

			// Assert
			assert.True(t, withoutOK)
			assert.False(t, withOK)
		})
	}

	for _, raw := range public {
		t.Run("allows "+raw, func(t *testing.T) {
			// Act
			blocked := blockedAddr(netip.MustParseAddr(raw), false)

			// Assert
			assert.False(t, blocked)
		})
	}
}

func Test_guardAddr(t *testing.T) {
	t.Run("refuses a blocked address", func(t *testing.T) {
		// Act
		err := guardAddr(context.Background(), "tcp", "127.0.0.1:443", nil)

		// Assert
		assert.ErrorIs(t, err, errBlockedAddr)
	})

	t.Run("passes privateOK from the context", func(t *testing.T) {
		// Arrange
		ctx := withPrivateOK(context.Background(), true)

		// Act
		err := guardAddr(ctx, "tcp", "10.0.0.1:443", nil)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("refuses a private address without privateOK", func(t *testing.T) {
		// Act
		err := guardAddr(context.Background(), "tcp", "10.0.0.1:443", nil)

		// Assert
		assert.ErrorIs(t, err, errBlockedAddr)
	})

	t.Run("fails closed on an unparsable address", func(t *testing.T) {
		// Act
		err := guardAddr(context.Background(), "tcp", "not-an-address", nil)

		// Assert
		assert.ErrorIs(t, err, errBlockedAddr)
	})
}
