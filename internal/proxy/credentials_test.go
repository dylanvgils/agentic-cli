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
		path := writeTestFile(t, "creds.json", `[{"hosts":["api.example.test"],"rules":[{"header":"X-Api-Key","value":"test-secret"}]}]`)

		// Act
		creds, err := LoadCredentials(path)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []Credential{{Hosts: []string{"api.example.test"}, Rules: []InjectRule{{Header: "X-Api-Key", Value: "test-secret"}}}}, creds)
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
			"no hosts":                 `[{"rules":[{"header":"X-Api-Key","value":"test-secret"}]}]`,
			"no rules":                 `[{"hosts":["api.example.test"]}]`,
			`invalid header name ""`:   `[{"hosts":["api.example.test"],"rules":[{"header":"","value":"test-secret"}]}]`,
			`invalid header name "a:"`: `[{"hosts":["api.example.test"],"rules":[{"header":"a:","value":"test-secret"}]}]`,
			"invalid value":            `[{"hosts":["api.example.test"],"rules":[{"header":"X-Api-Key","value":"test-secret\r\nX-Injected: 1"}]}]`,
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
