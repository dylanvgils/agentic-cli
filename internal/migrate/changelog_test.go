package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChangelog_isAscending(t *testing.T) {
	// Act + Assert
	for i := 1; i < len(changelog); i++ {
		assert.Greaterf(t, changelog[i].Version, changelog[i-1].Version,
			"changelog[%d] (v%d) must be greater than changelog[%d] (v%d) - entries must be ascending with no reused Version",
			i, changelog[i].Version, i-1, changelog[i-1].Version)
	}
}
