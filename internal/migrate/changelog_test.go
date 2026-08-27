package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChangelog_isSequential(t *testing.T) {
	// Act + Assert
	for i, m := range changelog {
		assert.Equalf(t, i+1, m.Version, "changelog[%d] has Version %d, want %d - versions must be sequential starting at 1", i, m.Version, i+1)
	}
}
