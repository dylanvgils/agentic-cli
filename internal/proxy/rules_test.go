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
		rules, err := LoadRules(`[{"host":".example.test"},{"host":"raw.example.test","paths":["/org/repo/**"]}]`)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []Rule{{Host: ".example.test"}, {Host: "raw.example.test", Paths: []string{"/org/repo/**"}}}, rules)
	})

	t.Run("refuses malformed or invalid rules", func(t *testing.T) {
		values := map[string]string{
			"parse":           `not json`,
			"invalid host":    `[{"host":"*.example.test","paths":["/a"]}]`,
			"must start with": `[{"host":"raw.example.test","paths":["a"]}]`,
		}
		for want, value := range values {
			// Act
			_, err := LoadRules(value)

			// Assert
			assert.ErrorContains(t, err, want, value)
		}
	})
}

func TestPathHosts(t *testing.T) {
	// Arrange
	rules := []Rule{
		{Host: "api.example.test"},
		{Host: "Raw.Example.Test.", Paths: []string{"/a"}},
		{Host: "raw.example.test", Paths: []string{"/b"}},
		{Host: "other.example.test", Paths: []string{"/c"}},
	}

	// Act
	hosts := PathHosts(rules)

	// Assert
	assert.Equal(t, []string{"raw.example.test", "other.example.test"}, hosts)
}
