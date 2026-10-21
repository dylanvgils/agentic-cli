package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBaseline(t *testing.T) {
	// Act
	err := Baseline(t.TempDir())

	// Assert
	require.NoError(t, err)
}
