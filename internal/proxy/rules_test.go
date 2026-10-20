package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRules(t *testing.T) {
	t.Run("empty value has no rules", func(t *testing.T) {
		// Act
		rules, err := LoadRules("")

		// Assert
		require.NoError(t, err)
		assert.Nil(t, rules)
	})

	t.Run("parses a valid rule list", func(t *testing.T) {
		// Act
		rules, err := LoadRules(`[{"host":".example.test"},{"host":"api.example.test"}]`)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []Rule{{Host: ".example.test"}, {Host: "api.example.test"}}, rules)
	})

	t.Run("refuses a malformed list", func(t *testing.T) {
		// Act
		_, err := LoadRules(`not json`)

		// Assert
		assert.ErrorContains(t, err, "parse")
	})
}
