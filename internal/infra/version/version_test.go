package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAtLeast(t *testing.T) {
	cases := map[string]struct {
		have, minimum string
		want          bool
	}{
		"equal":                      {"26.0", "26.0", true},
		"newer major":                {"29.9.0", "26.0", true},
		"older major":                {"20.10.24", "26.0", false},
		"missing parts count as 0":   {"26", "26.0.0", true},
		"newer patch":                {"0.7.10", "0.7.8", true},
		"older patch":                {"0.7.7", "0.7.8", false},
		"leading v":                  {"v0.7.8", "0.7.8", true},
		"suffix ends the version":    {"26.1.3-ce", "26.1.3", true},
		"suffix in an older version": {"25.0.0-rc.1", "26.0", false},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			got, err := AtLeast(c.have, c.minimum)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}

	t.Run("not a version is an error", func(t *testing.T) {
		// Act
		_, err := AtLeast("", "26.0")

		// Assert
		assert.EqualError(t, err, `not a version: ""`)
	})
}
