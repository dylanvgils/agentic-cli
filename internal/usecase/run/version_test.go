package run

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_requireVersion(t *testing.T) {
	t.Run("accepts the minimum", func(t *testing.T) {
		// Act
		err := requireVersion("Docker Engine", "26.0", "26.0")

		// Assert
		assert.NoError(t, err)
	})

	t.Run("refuses an older version", func(t *testing.T) {
		// Act
		err := requireVersion("Docker Engine", "25.0.5", "26.0")

		// Assert
		assert.EqualError(t, err, "Docker Engine 25.0.5 is older than the minimum 26.0; update it and run again")
	})

	t.Run("refuses an unreadable version", func(t *testing.T) {
		// Act
		err := requireVersion("Docker Engine", "dev", "26.0")

		// Assert
		assert.EqualError(t, err, `read the Docker Engine version: not a version: "dev"`)
	})
}

func Test_versionAtLeast(t *testing.T) {
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
			got, err := versionAtLeast(c.have, c.minimum)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}

	t.Run("not a version is an error", func(t *testing.T) {
		// Act
		_, err := versionAtLeast("", "26.0")

		// Assert
		assert.EqualError(t, err, `not a version: ""`)
	})
}
