package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRunID(t *testing.T) {
	// Act
	id, err := NewRunID()

	// Assert
	require.NoError(t, err)
	parsed, err := ParseRunID(id.String())
	require.NoError(t, err)
	assert.Equal(t, id, parsed)
}

func TestParseRunID(t *testing.T) {
	for _, tc := range []struct {
		name    string
		in      string
		wantErr string
	}{
		{
			name: "twelve hex chars",
			in:   "0123456789ab",
		},
		{
			name:    "empty",
			in:      "",
			wantErr: `invalid run id ""`,
		},
		{
			name:    "upper case",
			in:      "0123456789AB",
			wantErr: `invalid run id "0123456789AB"`,
		},
		{
			name:    "path traversal",
			in:      "../../etc/pa",
			wantErr: `invalid run id "../../etc/pa"`,
		},
		{
			name:    "too long",
			in:      "0123456789abc",
			wantErr: `invalid run id "0123456789abc"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			id, err := ParseRunID(tc.in)

			// Assert
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, RunID(tc.in), id)
		})
	}
}
