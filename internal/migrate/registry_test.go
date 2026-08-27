package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegistry_isSequential(t *testing.T) {
	// Act + Assert
	for i, m := range registry {
		assert.Equalf(t, i+1, m.Version, "registry[%d] has Version %d, want %d - versions must be sequential starting at 1", i, m.Version, i+1)
	}
}
