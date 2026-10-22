package proxy

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_scrubHeader(t *testing.T) {
	// Arrange
	header := http.Header{
		"X-Echo-Auth": {"Bearer test-secret"},
		"Location":    {"https://api.example.test/?token=test-secret", "test-secret,test-secret"},
	}

	// Act
	scrubHeader(header, strings.NewReplacer("test-secret", testPlaceholder))

	// Assert
	assert.Equal(t, http.Header{
		"X-Echo-Auth": {"Bearer " + testPlaceholder},
		"Location":    {"https://api.example.test/?token=" + testPlaceholder, testPlaceholder + "," + testPlaceholder},
	}, header)
}

func Test_newScrubber(t *testing.T) {
	cred := testPlaceholderCredential("api.example.test", testPlaceholder)
	basic := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

	t.Run("swaps the secret back to the placeholder", func(t *testing.T) {
		// Arrange
		scrub := newScrubber(http.Header{}, cred)

		// Act
		got := scrub.Replace("echo: Bearer test-secret")

		// Assert
		assert.Equal(t, "echo: Bearer "+testPlaceholder, got)
	})

	t.Run("swaps the basic-encoded secret back to the encoded placeholder the tool sent", func(t *testing.T) {
		// Arrange
		scrub := newScrubber(http.Header{"Authorization": {"Basic " + basic("x-access-token:"+testPlaceholder)}}, cred)

		// Act
		got := scrub.Replace(`{"auth":"Basic ` + basic("x-access-token:test-secret") + `"}`)

		// Assert
		assert.Equal(t, `{"auth":"Basic `+basic("x-access-token:"+testPlaceholder)+`"}`, got)
	})

	t.Run("leaves basic values in headers the credential doesn't list alone", func(t *testing.T) {
		// Arrange
		scrub := newScrubber(http.Header{"X-Other": {"Basic " + basic("user:"+testPlaceholder)}}, cred)

		// Act
		got := scrub.Replace(basic("user:test-secret"))

		// Assert
		assert.Equal(t, basic("user:test-secret"), got)
	})

	t.Run("leaves values alone for an empty secret", func(t *testing.T) {
		// Arrange
		scrub := newScrubber(http.Header{}, Credential{Placeholder: testPlaceholder})

		// Act
		got := scrub.Replace("text/plain")

		// Assert
		assert.Equal(t, "text/plain", got)
	})
}

func Test_substitute(t *testing.T) {
	cred := testPlaceholderCredential("api.example.test", testPlaceholder)

	t.Run("swaps the placeholder in every value of a listed header", func(t *testing.T) {
		// Arrange
		header := http.Header{"Authorization": {"Bearer agentic-0123456789abcdef", "Token agentic-0123456789abcdef"}}

		// Act
		substitute(header, cred)

		// Assert
		assert.Equal(t, []string{"Bearer test-secret", "Token test-secret"}, header.Values("Authorization"))
	})

	t.Run("leaves headers the credential doesn't list untouched", func(t *testing.T) {
		// Arrange
		header := http.Header{"X-Other": {testPlaceholder}}

		// Act
		substitute(header, cred)

		// Assert
		assert.Equal(t, testPlaceholder, header.Get("X-Other"))
	})

	t.Run("drops aliases of a listed header and keeps near misses", func(t *testing.T) {
		// Arrange
		header := http.Header{
			"X-Api-Key":  {testPlaceholder},
			"X_api_key":  {testPlaceholder},
			"X_API-KEY":  {"mixed"},
			"X-Api-Keys": {"other"},
		}

		// Act
		substitute(header, cred)

		// Assert
		assert.Equal(t, http.Header{"X-Api-Key": {"test-secret"}, "X-Api-Keys": {"other"}}, header)
	})
}

func Test_hasPlaceholder(t *testing.T) {
	cred := Credential{Headers: []string{"Authorization"}, Placeholder: testPlaceholder, Secret: "test-secret"}
	basic := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + testPlaceholder))
	cases := map[string]struct {
		header http.Header
		want   bool
	}{
		"plain placeholder in a listed header":  {http.Header{"Authorization": {"Bearer " + testPlaceholder}}, true},
		"basic-encoded placeholder":             {http.Header{"Authorization": {"Basic " + basic}}, true},
		"listed header without the placeholder": {http.Header{"Authorization": {"Bearer own-token"}}, false},
		"placeholder in an unlisted header":     {http.Header{"X-Api-Key": {testPlaceholder}}, false},
		"no headers":                            {http.Header{}, false},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			got := hasPlaceholder(c.header, cred)

			// Assert
			assert.Equal(t, c.want, got)
		})
	}
}

func Test_substituteValue(t *testing.T) {
	placeholder := testPlaceholder
	basic := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	cases := map[string]struct{ value, want string }{
		"bare value":                           {placeholder, "test-secret"},
		"bearer token":                         {"Bearer " + placeholder, "Bearer test-secret"},
		"every occurrence":                     {placeholder + "," + placeholder, "test-secret,test-secret"},
		"basic credential is decoded":          {"Basic " + basic("x-access-token:"+placeholder), "Basic " + basic("x-access-token:test-secret")},
		"basic scheme in any case":             {"basic " + basic("user:"+placeholder), "basic " + basic("user:test-secret")},
		"basic credential without placeholder": {"Basic " + basic("user:pass"), "Basic " + basic("user:pass")},
		"basic with a plain-text placeholder":  {"Basic " + placeholder, "Basic test-secret"},
		"no placeholder":                       {"Bearer own-token", "Bearer own-token"},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			got := substituteValue(c.value, placeholder, "test-secret")

			// Assert
			assert.Equal(t, c.want, got)
		})
	}
}
