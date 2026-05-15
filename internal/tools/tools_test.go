package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNames_sorted(t *testing.T) {
	// Act
	names := Names()

	// Assert
	assert.Equal(t, []string{"claude", "copilot", "opencode"}, names)
}

func TestNames_matchConfigKeys(t *testing.T) {
	// Act
	names := Names()

	// Assert
	assert.Len(t, names, len(Configs))
	for _, n := range names {
		assert.Contains(t, Configs, n)
	}
}
