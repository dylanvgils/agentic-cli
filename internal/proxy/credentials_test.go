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

	t.Run("wildcard and malformed hosts are rejected", func(t *testing.T) {
		for _, host := range []string{"*.example.test", ".example.test", "*.", ".", "", "api..example.test", "-api.example.test", "api_key.example.test", "api.example.test:443"} {
			// Arrange
			path := writeTestFile(t, "creds.json", `[{"hosts":["`+host+`"],"rules":[{"header":"X-Api-Key","value":"test-secret"}]}]`)

			// Act
			_, err := LoadCredentials(path)

			// Assert
			assert.ErrorContains(t, err, "invalid host", host)
		}
	})

	t.Run("exact hostnames and ips are accepted", func(t *testing.T) {
		// Arrange
		path := writeTestFile(t, "creds.json", `[{"hosts":["API.example.test.","127.0.0.1","::1"],"rules":[{"header":"X-Api-Key","value":"test-secret"}]}]`)

		// Act
		_, err := LoadCredentials(path)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("host listed in two credentials is rejected", func(t *testing.T) {
		// Arrange
		path := writeTestFile(t, "creds.json", `[{"hosts":["api.example.test"],"rules":[{"header":"X-Api-Key","value":"a"}]},{"hosts":["API.example.test."],"rules":[{"header":"X-Api-Key","value":"b"}]}]`)

		// Act
		_, err := LoadCredentials(path)

		// Assert
		assert.ErrorContains(t, err, "listed in more than one credential")
	})

	t.Run("invalid headers are rejected", func(t *testing.T) {
		cases := map[string]string{
			`invalid header name "X Api"`:            `{"header":"X Api","value":"test-secret"}`,
			`invalid header name "X(Api)"`:           `{"header":"X(Api)","value":"test-secret"}`,
			`invalid header name "X-Ápi"`:            `{"header":"X-Ápi","value":"test-secret"}`,
			"header host cannot be injected":         `{"header":"host","value":"evil.test"}`,
			"header Connection cannot be injected":   `{"header":"Connection","value":"close"}`,
			"invalid value for header X-Nul":         `{"header":"X-Nul","value":"a\u0000b"}`,
			"invalid value for header X-Del":         `{"header":"X-Del","value":"a\u007fb"}`,
			"invalid value for header X-Empty-Value": `{"header":"X-Empty-Value","value":""}`,
		}
		for want, rule := range cases {
			// Arrange
			path := writeTestFile(t, "creds.json", `[{"hosts":["api.example.test"],"rules":[`+rule+`]}]`)

			// Act
			_, err := LoadCredentials(path)

			// Assert
			assert.ErrorContains(t, err, want)
		}
	})
}

func TestValidateCredentials(t *testing.T) {
	t.Run("valid credentials pass", func(t *testing.T) {
		// Arrange
		creds := []Credential{{Hosts: []string{"api.example.test"}, Rules: []InjectRule{{Header: "X-Api-Key", Value: "test-secret"}}}}

		// Act
		err := ValidateCredentials(creds)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("error does not contain the header value", func(t *testing.T) {
		// Arrange
		creds := []Credential{{Hosts: []string{"api.example.test"}, Rules: []InjectRule{{Header: "X-Api-Key", Value: "test-secret\n"}}}}

		// Act
		err := ValidateCredentials(creds)

		// Assert
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "test-secret")
	})
}
