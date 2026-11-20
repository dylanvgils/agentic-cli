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

	t.Run("an invalid credential is an error", func(t *testing.T) {
		// Arrange
		path := writeCredentials(t, Credential{Hosts: []string{"api.example.test"}})

		// Act
		_, err := LoadCredentials(path)

		// Assert
		assert.ErrorContains(t, err, "proxy credential 0: no headers")
	})
}

func TestValidateCredentials(t *testing.T) {
	t.Run("exact hostnames and ips pass", func(t *testing.T) {
		// Arrange
		cred := testPlaceholderCredential("API.example.test.", testPlaceholder)
		cred.Hosts = append(cred.Hosts, "127.0.0.1", "::1")

		// Act
		err := ValidateCredentials([]Credential{cred})

		// Assert
		assert.NoError(t, err)
	})

	for _, tc := range []struct {
		name    string
		breakIt func(c *Credential)
		want    string
	}{
		{name: "no hosts", breakIt: func(c *Credential) { c.Hosts = nil }, want: "no hosts"},
		{name: "a bare wildcard host", breakIt: func(c *Credential) { c.Hosts = []string{"*."} }, want: `invalid host "*."`},
		{name: "a root-only host", breakIt: func(c *Credential) { c.Hosts = []string{"."} }, want: `invalid host "."`},
		{name: "no headers", breakIt: func(c *Credential) { c.Headers = nil }, want: "no headers"},
		{name: "an empty header", breakIt: func(c *Credential) { c.Headers = []string{""} }, want: `invalid header name ""`},
		{name: "a header with a colon", breakIt: func(c *Credential) { c.Headers = []string{"a:"} }, want: `invalid header name "a:"`},
		{name: "a header with a space", breakIt: func(c *Credential) { c.Headers = []string{"X Api"} }, want: `invalid header name "X Api"`},
		{name: "a header with parentheses", breakIt: func(c *Credential) { c.Headers = []string{"X(Api)"} }, want: `invalid header name "X(Api)"`},
		{name: "a non-ascii header", breakIt: func(c *Credential) { c.Headers = []string{"X-Ápi"} }, want: `invalid header name "X-Ápi"`},
		{name: "the host header", breakIt: func(c *Credential) { c.Headers = []string{"Host"} }, want: "header Host cannot be injected"},
		{name: "a lowercase host header", breakIt: func(c *Credential) { c.Headers = []string{"host"} }, want: "header host cannot be injected"},
		{name: "a hop-by-hop header", breakIt: func(c *Credential) { c.Headers = []string{"Connection"} }, want: "header Connection cannot be injected"},
		{name: "a short placeholder", breakIt: func(c *Credential) { c.Placeholder = "agentic-short" }, want: "placeholder must be at least 16"},
		{name: "a placeholder with a space", breakIt: func(c *Credential) { c.Placeholder = "agentic 0123456789abcdef" }, want: "letters, digits, - or _"},
		{name: "a secret with a newline", breakIt: func(c *Credential) { c.Secret = "test-secret\n" }, want: "invalid secret"},
		{name: "a secret with a nul", breakIt: func(c *Credential) { c.Secret = "test-secret\u0000" }, want: "invalid secret"},
		{name: "a secret with a del", breakIt: func(c *Credential) { c.Secret = "test-secret\u007f" }, want: "invalid secret"},
		{name: "an empty secret", breakIt: func(c *Credential) { c.Secret = "" }, want: "invalid secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			cred := testPlaceholderCredential("api.example.test", testPlaceholder)
			tc.breakIt(&cred)

			// Act
			err := ValidateCredentials([]Credential{cred})

			// Assert
			require.ErrorContains(t, err, tc.want)
			assert.NotContains(t, err.Error(), "test-secret", "errors never hold the secret")
		})
	}

	t.Run("host listed in two credentials is rejected", func(t *testing.T) {
		// Arrange
		first := testPlaceholderCredential("api.example.test", testPlaceholder)
		first.Placeholder, first.Secret = "agentic-aaaaaaaaaaaaaaaa", "a"
		second := testPlaceholderCredential("API.example.test.", testPlaceholder)
		second.Placeholder, second.Secret = "agentic-bbbbbbbbbbbbbbbb", "b"

		// Act
		err := ValidateCredentials([]Credential{first, second})

		// Assert
		assert.ErrorContains(t, err, "listed in more than one credential")
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
