package proxy

import (
	"bytes"
	"fmt"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCredentials(t *testing.T) {
	t.Run("parses a placeholder credential", func(t *testing.T) {
		// Arrange
		path := writeTestFile(t, "creds.json", `[{"hosts":["api.example.test"],"headers":["Authorization"],"placeholder":"agentic-0123456789abcdef","secret":"test-secret"}]`)

		// Act
		creds, err := LoadCredentials(path)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []Credential{{
			Hosts:       []string{"api.example.test"},
			Headers:     []string{"Authorization"},
			Placeholder: "agentic-0123456789abcdef",
			Secret:      "test-secret",
		}}, creds)
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
			"no hosts":                 `[{"headers":["X-Api-Key"],"placeholder":"agentic-0123456789abcdef","secret":"test-secret"}]`,
			"no headers":               `[{"hosts":["api.example.test"],"placeholder":"agentic-0123456789abcdef","secret":"test-secret"}]`,
			`invalid header name ""`:   `[{"hosts":["api.example.test"],"headers":[""],"placeholder":"agentic-0123456789abcdef","secret":"test-secret"}]`,
			`invalid header name "a:"`: `[{"hosts":["api.example.test"],"headers":["a:"],"placeholder":"agentic-0123456789abcdef","secret":"test-secret"}]`,
			"placeholder must be":      `[{"hosts":["api.example.test"],"headers":["X-Api-Key"],"secret":"test-secret"}]`,
			"invalid secret":           `[{"hosts":["api.example.test"],"headers":["X-Api-Key"],"placeholder":"agentic-0123456789abcdef","secret":"test-secret\r\nX-Injected: 1"}]`,
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
			path := writeTestFile(t, "creds.json", `[{"hosts":["`+host+`"],"headers":["X-Api-Key"],"placeholder":"agentic-0123456789abcdef","secret":"test-secret"}]`)

			// Act
			_, err := LoadCredentials(path)

			// Assert
			assert.ErrorContains(t, err, "invalid host", host)
		}
	})

	t.Run("exact hostnames and ips are accepted", func(t *testing.T) {
		// Arrange
		path := writeTestFile(t, "creds.json", `[{"hosts":["API.example.test.","127.0.0.1","::1"],"headers":["X-Api-Key"],"placeholder":"agentic-0123456789abcdef","secret":"test-secret"}]`)

		// Act
		_, err := LoadCredentials(path)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("host listed in two credentials is rejected", func(t *testing.T) {
		// Arrange
		path := writeTestFile(t, "creds.json", `[{"hosts":["api.example.test"],"headers":["X-Api-Key"],"placeholder":"agentic-aaaaaaaaaaaaaaaa","secret":"a"},{"hosts":["API.example.test."],"headers":["X-Api-Key"],"placeholder":"agentic-bbbbbbbbbbbbbbbb","secret":"b"}]`)

		// Act
		_, err := LoadCredentials(path)

		// Assert
		assert.ErrorContains(t, err, "listed in more than one credential")
	})

	t.Run("invalid headers are rejected", func(t *testing.T) {
		cases := map[string]string{
			`invalid header name "X Api"`:          `"X Api"`,
			`invalid header name "X(Api)"`:         `"X(Api)"`,
			`invalid header name "X-Ápi"`:          `"X-Ápi"`,
			"header host cannot be injected":       `"host"`,
			"header Connection cannot be injected": `"Connection"`,
		}
		for want, header := range cases {
			// Arrange
			path := writeTestFile(t, "creds.json", `[{"hosts":["api.example.test"],"headers":[`+header+`],"placeholder":"agentic-0123456789abcdef","secret":"test-secret"}]`)

			// Act
			_, err := LoadCredentials(path)

			// Assert
			assert.ErrorContains(t, err, want)
		}
	})
}

func TestValidateCredential(t *testing.T) {
	t.Run("secrets a header can't carry are rejected", func(t *testing.T) {
		for _, secret := range []string{"a\u0000b", "a\u007fb", "", "line\nbreak"} {
			// Arrange
			cred := testPlaceholderCredential("api.example.test", "agentic-0123456789abcdef")
			cred.Secret = secret

			// Act
			err := ValidateCredential(cred)

			// Assert
			assert.ErrorContains(t, err, "invalid secret", secret)
		}
	})
}

func TestValidateCredentials(t *testing.T) {
	t.Run("valid credentials pass", func(t *testing.T) {
		// Arrange
		creds := []Credential{testPlaceholderCredential("api.example.test", "agentic-0123456789abcdef")}

		// Act
		err := ValidateCredentials(creds)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("error does not contain the secret", func(t *testing.T) {
		// Arrange
		cred := testPlaceholderCredential("api.example.test", "agentic-0123456789abcdef")
		cred.Secret = "test-secret\n"
		creds := []Credential{cred}

		// Act
		err := ValidateCredentials(creds)

		// Assert
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "test-secret")
	})
	t.Run("invalid credentials are rejected", func(t *testing.T) {
		valid := testPlaceholderCredential("api.example.test", "agentic-0123456789abcdef")
		noHeaders, badHeader, reserved, short, badChars, badSecret := valid, valid, valid, valid, valid, valid
		noHeaders.Headers = nil
		badHeader.Headers = []string{"X Api"}
		reserved.Headers = []string{"Host"}
		short.Placeholder = "agentic-short"
		badChars.Placeholder = "agentic 0123456789abcdef"
		badSecret.Secret = "test-secret\n"
		cases := map[string]Credential{
			"no headers":                      noHeaders,
			`invalid header name "X Api"`:     badHeader,
			"header Host cannot be injected":  reserved,
			"placeholder must be at least 16": short,
			"letters, digits, - or _":         badChars,
			"invalid secret":                  badSecret,
		}

		for want, cred := range cases {
			// Act
			err := ValidateCredentials([]Credential{cred})

			// Assert
			assert.ErrorContains(t, err, want)
			if err != nil {
				assert.NotContains(t, err.Error(), "test-secret", want)
			}
		}
	})

	t.Run("overlapping placeholders are rejected", func(t *testing.T) {
		// Arrange
		creds := []Credential{
			testPlaceholderCredential("api.example.test", "agentic-0123456789abcdef"),
			testPlaceholderCredential("git.example.test", "agentic-0123456789abcdef-2"),
		}

		// Act
		err := ValidateCredentials(creds)

		// Assert
		assert.ErrorContains(t, err, "overlap")
	})
}

func TestCredential(t *testing.T) {
	cred := testPlaceholderCredential("api.example.test", "agentic-0123456789abcdef")

	t.Run("fmt verbs hide the secret", func(t *testing.T) {
		for _, verb := range []string{"%v", "%+v", "%#v", "%s"} {
			// Act
			out := fmt.Sprintf(verb, cred)

			// Assert
			assert.NotContains(t, out, "test-secret", verb)
			assert.Contains(t, out, "agentic-0123456789abcdef", verb)
		}
	})

	t.Run("slog hides the secret", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))

		// Act
		logger.Info("credential", "credential", cred)

		// Assert
		assert.NotContains(t, buf.String(), "test-secret")
		assert.Contains(t, buf.String(), "api.example.test")
	})
}

func TestCredentialHosts(t *testing.T) {
	// Arrange
	creds := []Credential{
		{Hosts: []string{"API.example.test.", "a.example.test"}},
		{Hosts: []string{"api.example.test", "b.example.test"}},
	}

	// Act
	hosts := CredentialHosts(creds)

	// Assert
	assert.Equal(t, []string{"api.example.test", "a.example.test", "b.example.test"}, hosts)
}
