package proxy

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCredentials(t *testing.T) {
	t.Run("parses hosts and rules", func(t *testing.T) {
		// Arrange
		path := writeTestFile(t, "creds.json", `[{"hosts":["api.anthropic.com"],"rules":[{"header":"x-api-key","value":"sk-test"}]}]`)

		// Act
		creds, err := LoadCredentials(path)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []Credential{{Hosts: []string{"api.anthropic.com"}, Rules: []InjectRule{{Header: "x-api-key", Value: "sk-test"}}}}, creds)
	})

	t.Run("missing file is an error", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "missing.json")

		// Act
		_, err := LoadCredentials(path)

		// Assert
		assert.ErrorContains(t, err, "read proxy credentials")
	})

	t.Run("invalid json is an error", func(t *testing.T) {
		// Arrange
		path := writeTestFile(t, "creds.json", `{"hosts":`)

		// Act
		_, err := LoadCredentials(path)

		// Assert
		assert.ErrorContains(t, err, "parse proxy credentials")
	})

	t.Run("incomplete credentials are rejected", func(t *testing.T) {
		cases := map[string]string{
			"no hosts":                 `[{"rules":[{"header":"x-api-key","value":"sk"}]}]`,
			"no rules":                 `[{"hosts":["a.test"]}]`,
			`invalid header name ""`:   `[{"hosts":["a.test"],"rules":[{"header":"","value":"sk"}]}]`,
			`invalid header name "a:"`: `[{"hosts":["a.test"],"rules":[{"header":"a:","value":"sk"}]}]`,
			"invalid value":            `[{"hosts":["a.test"],"rules":[{"header":"x-api-key","value":"sk\r\nX-Evil: 1"}]}]`,
		}
		for want, content := range cases {
			// Arrange
			path := writeTestFile(t, "creds.json", content)

			// Act
			_, err := LoadCredentials(path)

			// Assert
			assert.ErrorContains(t, err, want)
		}
	})
}
