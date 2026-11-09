package dind

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddressPool(t *testing.T) {
	for _, tc := range []struct {
		name       string
		runSubnets []string
		want       string
	}{
		{
			name:       "docker default subnet takes the first candidate",
			runSubnets: []string{"172.19.0.0/16"},
			want:       "base=10.200.0.0/16,size=24",
		},
		{
			name:       "overlapping subnet skips a candidate",
			runSubnets: []string{"10.200.5.0/24"},
			want:       "base=10.201.0.0/16,size=24",
		},
		{
			name:       "wide subnet skips every candidate it covers",
			runSubnets: []string{"10.200.0.0/15", "fd00::/64"},
			want:       "base=10.202.0.0/16,size=24",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			pool, err := AddressPool(tc.runSubnets)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, pool)
		})
	}

	t.Run("refuses when every candidate overlaps", func(t *testing.T) {
		// Act
		_, err := AddressPool([]string{"10.0.0.0/8"})

		// Assert
		require.EqualError(t, err, "no inner address pool is free of the run network subnets [10.0.0.0/8]")
	})

	t.Run("refuses an unparsable subnet", func(t *testing.T) {
		// Act
		_, err := AddressPool([]string{"not-a-subnet"})

		// Assert
		require.ErrorContains(t, err, "parse run network subnet")
	})
}
